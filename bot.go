package bote

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/maxbolgarin/abstract"
	"github.com/maxbolgarin/errm"
	"github.com/maxbolgarin/lang"
	tele "gopkg.in/telebot.v4"
)

// InitBundle is a struct for initing users of the bot. You should provide it to [Bot.Start] method.
// You should create a mapping of every [State] to the corresponding [HandlerFunc].
type InitBundle struct {
	// Handler is a handler for [State]. It will be called for initing user.
	// It is required.
	Handler HandlerFunc
	// Data is a callback data, that can be obtained from [Context.Data] inside [HandlerFunc].
	// It is optional and shoud be provided if you use [Context.Data] in your [InitBundle.Handler].
	Data string
	// Text is a text of simulating message, that can be obtained from [Context.Text] inside [HandlerFunc].
	// It is optional and shoud be provided if you use [Context.Text] in your [InitBundle.Handler].
	Text string
	// State is a state, that will be set for user after [InitBundle.Handler] is called.
	// It is optional and shoud be provided if you don't want to live with the state that will be set in [InitBundle.Handler].
	State State
}

// Bot is a main struct of this package. It contains all necessary components for working with Telegram bot.
type Bot struct {
	bot  *baseBot
	um   *userManagerImpl
	msgs MessageProvider
	rlog UpdateLogger

	defaultLanguageCode string

	middlewares    *abstract.SafeSlice[MiddlewareFunc]
	stateMap       *abstract.SafeMap[string, InitBundle]
	startHandler   HandlerFunc
	deleteMessages bool
	logUpdates     bool
}

// New creates the bot with optional options.
func New(ctx context.Context, token string, optsFuncs ...func(*Options)) (*Bot, error) {
	var opts Options
	for _, f := range optsFuncs {
		f(&opts)
	}
	return NewWithOptions(ctx, token, opts)
}

// NewWithOptions starts the bot with options.
func NewWithOptions(ctx context.Context, token string, opts Options) (*Bot, error) {
	if token == "" {
		return nil, errm.New("token cannot be empty")
	}
	opts, err := prepareOpts(opts)
	if err != nil {
		return nil, errm.Wrap(err, "prepare opts")
	}

	um, err := newUserManager(opts.UserDB, opts.Logger, opts.Config.UserCacheCapacity, opts.Config.UserCacheTTL)
	if err != nil {
		return nil, errm.Wrap(err, "new user manager")
	}

	b, err := newBaseBot(token, opts)
	if err != nil {
		return nil, errm.Wrap(err, "start bot")
	}

	bote := &Bot{
		bot:  b,
		um:   um,
		msgs: opts.Msgs,
		rlog: opts.UpdateLogger,

		defaultLanguageCode: opts.Config.DefaultLanguageCode,
		middlewares:         abstract.NewSafeSlice[MiddlewareFunc](),
		stateMap:            abstract.NewSafeMap[string, InitBundle](),
		deleteMessages:      *opts.Config.DeleteMessages,
		logUpdates:          *opts.Config.LogUpdates,
	}

	b.addMiddleware(bote.masterMiddleware)
	bote.AddMiddleware(bote.cleanMiddleware)
	bote.AddMiddleware(bote.startMiddleware)

	// To trigger initUserHandler on callback (there is no registered handlers if user is not inited)
	bote.Handle(tele.OnCallback, bote.emptyHandler)

	return bote, nil
}

// Start starts the bot in a separate goroutine.
// StartHandler is a handler for /start command.
// StateMap is a map for initing users after bot restart.
// It runs an assigned handler for every active user message when user makes a request by message's state.
// Inline buttons will trigger onCallback handler if you don't init them after bot restart.
// You can pass nil map if you don't need to reinit messages.
// Don't forget to call Stop() to gracefully shutdown the bot.
func (b *Bot) Start(ctx context.Context, startHandler HandlerFunc, stateMap map[State]InitBundle) {
	b.startHandler = startHandler
	for k, v := range stateMap {
		b.stateMap.Set(k.String(), v)
	}
	b.Handle(startCommand, b.startHandler)
	b.bot.start()
}

// Stop gracefully shuts the poller down.
func (b *Bot) Stop() {
	b.bot.stop()
}

// Bot returns the underlying *tele.Bot.
func (b *Bot) Bot() *tele.Bot {
	return b.bot.tbot
}

// GetUser returns user by its ID.
func (b *Bot) GetUser(userID int64) User {
	return b.um.getUser(userID)
}

// GetAllUsers returns all loaded users.
func (b *Bot) GetAllUsers() []User {
	return b.um.getAllUsers()
}

// AddMiddleware adds middleware functions that will be called on each update.
func (b *Bot) AddMiddleware(f ...MiddlewareFunc) {
	b.middlewares.Append(f...)
}

// Handle sets handler for any endpoint. Endpoint can be string or callback button.
func (b *Bot) Handle(endpoint any, f HandlerFunc) {
	b.bot.handle(endpoint, func(c tele.Context) (err error) {
		defer lang.RecoverWithErrAndStack(b.bot.log, &err)

		ctx := b.newContext(c)
		defer func() {
			if b.logUpdates {
				upd := c.Update()
				b.logUpdate(&upd, ctx.user)
			}
		}()

		msgID := ctx.MessageID()
		if !ctx.user.isMsgInited(msgID) && b.stateMap.Len() > 0 {
			if err := b.initUserHandler(ctx, msgID); err != nil {
				return ctx.handleError(err)
			}
		}

		if ep, ok := endpoint.(string); ok && ep == tele.OnText {
			if !ctx.user.hasTextMessages() {
				return nil
			}
			ctx.textMsgID = ctx.user.lastTextMessage()

			// /start was already handled
			if ctx.ct.Text() == startCommand {
				return nil
			}
		}

		if err = f(ctx); err != nil {
			return ctx.handleError(err)
		}

		if c.Callback() != nil {
			c.Respond(&tele.CallbackResponse{})
		}

		return nil
	})
}

// SetTextHandler sets handler for text messages.
// You should provide a single handler for all text messages, that will call another handlers based on the state.
func (b *Bot) SetTextHandler(handler HandlerFunc) {
	b.Handle(tele.OnText, handler)
}

// SetMessageProvider sets message provider.
func (b *Bot) SetMessageProvider(msgs MessageProvider) {
	b.msgs = msgs
}

func (b *Bot) initUserHandler(ctx *contextImpl, msgID int) error {
	defer ctx.user.setMsgInited(msgID)
	if ctx.user.Messages().MainID == 0 || ctx.user.StateMain() == FirstRequest {
		return nil
	}

	if err := b.initUserMsg(ctx.user, msgID); err != nil {
		return err
	}

	btnID := ctx.ButtonID()
	if btnID == "" {
		return nil
	}

	targetHandler, ok := ctx.user.buttonMap.Lookup(btnID)
	if !ok {
		b.bot.log.Warn("button handler not found", "user_id", ctx.user.ID(), "button_id", btnID)
		return nil
	}

	upd := tele.Update{
		Callback: &tele.Callback{
			Message: &tele.Message{ID: ctx.MessageID()},
			Data:    targetHandler.Data,
		},
	}

	return targetHandler.Handler(&contextImpl{
		bt:   b,
		ct:   b.bot.tbot.NewContext(upd),
		user: ctx.user,
	})
}

func (b *Bot) emptyHandler(ctx Context) error {
	return nil
}

func (b *Bot) masterMiddleware(upd *tele.Update) bool {
	defer lang.Recover(b.bot.log)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender := getSender(upd)
	if sender == nil {
		b.bot.log.Error(fmt.Sprintf("cannot get sender from update: %+v", upd))
		return false
	}

	user, err := b.um.prepareUser(ctx, sender)
	if err != nil {
		b.bot.log.Error("cannot prepare user", "error", err, "user_id", sender.ID, "username", sender.Username)
		b.sendError(sender.ID, b.msgs.Messages(b.defaultLanguageCode).GeneralError())
		return false
	}

	return b.middlewares.Range(func(mf MiddlewareFunc) bool {
		return mf(upd, user)
	})
}

func (b *Bot) cleanMiddleware(upd *tele.Update, userRaw User) bool {
	user := userRaw.(*userContextImpl)

	msgIDs := user.Messages()
	if msgIDs.ErrorID > 0 {
		b.bot.delete(user.ID(), msgIDs.ErrorID)
		user.setErrorMessage(0)
	}
	if upd.Message != nil && b.deleteMessages {
		if upd.Message.Text == startCommand && user.Messages().MainID == 0 {
			return true
		}
		b.bot.delete(user.ID(), upd.Message.ID)
	}

	if upd.Callback != nil {
		if match := cbackRx.FindAllStringSubmatch(upd.Callback.Data, -1); match != nil {
			user.setBtnName(getNameFromUnique(match[0][1]))
			user.setPayload(match[0][3])
		}
	}

	// Sanitize incoming message text to prevent injection attacks
	if upd.Message != nil && upd.Message.Text != "" {
		upd.Message.Text = sanitizeText(upd.Message.Text, 1e5)
	}

	return true
}

var cbackRx = regexp.MustCompile(`([-\w]+)(\|(.+))?`)

func (b *Bot) logUpdate(upd *tele.Update, userRaw User) bool {
	user := userRaw.(*userContextImpl)

	fields := make([]any, 0, 7)
	fields = append(fields,
		"user_id", user.ID(),
		"username", user.Username(),
	)

	switch {
	case upd.Message != nil:
		fields = append(fields, "state", user.StateMain(), "msg_id", upd.Message.ID, "text", maxLen(upd.Message.Text, MaxTextLenInLogs))
		if user.hasTextMessages() {
			msgID := user.lastTextMessage()
			ts, _ := user.State(msgID)
			fields = append(fields, "text_state", ts, "text_state_msg_id", msgID)
		}
		b.rlog.Log(MessageUpdate, fields...)

	case upd.Callback != nil:
		if upd.Callback.Message != nil {
			st, _ := user.State(upd.Callback.Message.ID)
			if st.NotChanged() {
				st = user.StateMain()
			}
			fields = append(fields, "state", st, "msg_id", upd.Callback.Message.ID)
		} else {
			fields = append(fields, "state", user.StateMain())
		}

		if btnName := user.getBtnName(); btnName != "" {
			fields = append(fields, "button", any(btnName))
		}
		if payload := user.getPayload(); payload != "" {
			fields = append(fields, "payload", any(payload))
		}

		b.rlog.Log(CallbackUpdate, fields...)
		return true
	}

	return true
}

func (b *Bot) startMiddleware(upd *tele.Update, userRaw User) bool {
	if upd.Message != nil && upd.Message.Text == startCommand {
		return true
	}

	st := userRaw.StateMain()
	switch st {
	case FirstRequest:
		if b.startHandler != nil {
			b.startHandler(b.newContextFromUpdate(*upd))
		}
	}
	return true
}

func userFields(user User, fields ...any) []any {
	f := make([]any, 0, len(fields)+6)
	f = append(f, "user_id", user.ID(), "username", user.Username(), "state", user.StateMain())
	return append(f, fields...)
}

func (b *Bot) initUserMsg(user *userContextImpl, msgID int) error {
	st, ok := user.State(msgID)
	if !ok {
		b.bot.log.Debug("forget history message without state", "user_id", user.ID(), "msg_id", msgID)
		user.forgetHistoryMessage(msgID)
		return nil
	}

	bundle, foundBundle := b.stateMap.Lookup(st.String())
	if !foundBundle {
		b.bot.log.Warn("init bundle not found", "user_id", user.ID(), "msg_id", msgID, "state", st)
		bundle = InitBundle{
			Handler: b.startHandler,
		}
	}

	b.bot.log.Debug("init user message", "user_id", user.ID(), "msg_id", msgID, "state", st)

	targetBundleErr := b.init(bundle, user, msgID, st)
	if targetBundleErr != nil {
		if !foundBundle {
			// got error by startHandler
			return errm.Wrap(targetBundleErr, "start handler", "msg_id", msgID, "state", st)
		}
		startHandlerErr := b.init(InitBundle{
			Handler: b.startHandler,
		}, user, msgID, st)
		if startHandlerErr != nil {
			return errm.Wrap(startHandlerErr, "start handler", "msg_id", msgID, "state", st, "first_error", targetBundleErr)
		}
	}

	return nil
}

func (b *Bot) init(bundle InitBundle, user *userContextImpl, msgID int, expectedState State) error {
	// Minimum update to handle all possible methods in [Context]
	upd := tele.Update{
		Message: &tele.Message{
			Text: bundle.Text,
			Sender: &tele.User{
				ID: user.ID(),
			},
		},
		Callback: &tele.Callback{
			Message: &tele.Message{ID: msgID},
			Data:    bundle.Data,
		},
	}

	mainBefore := user.Messages().MainID
	headBefore := user.Messages().HeadID

	err := bundle.Handler(&contextImpl{
		bt:   b,
		ct:   b.bot.tbot.NewContext(upd),
		user: user,
	})
	if err != nil {
		if strings.Contains(err.Error(), "is not modified") {
			b.bot.log.Debug("message is not modified in init", "user_id", user.ID(), "msg_id", msgID)
			return nil
		}
		return errm.Wrap(err, "run handler")
	}

	newMainID := user.Messages().MainID
	newHeadID := user.Messages().HeadID

	if mainBefore != newMainID {
		b.bot.log.Debug("main message id changed in init", "user_id", user.ID(), "msg_id", msgID)
		if err := b.bot.delete(user.ID(), mainBefore); err != nil {
			b.bot.log.Error("error deleting old main message", "user_id", user.ID(), "msg_id", mainBefore, "error", err)
		}
		user.forgetHistoryMessage(mainBefore)
		msgID = newMainID
	}
	if headBefore != newHeadID {
		b.bot.log.Debug("head message id changed in init", "user_id", user.ID(), "msg_id", msgID)
		if err := b.bot.delete(user.ID(), headBefore); err != nil {
			b.bot.log.Error("error deleting old head message", "user_id", user.ID(), "msg_id", headBefore, "error", err)
		}
		user.forgetHistoryMessage(headBefore)
	}

	// Expected state not changed after running handler
	if bundle.State == nil || bundle.State.NotChanged() {
		newState, ok := user.State(msgID)
		if !ok {
			return errm.New("new state not found")
		}
		if newState != expectedState {
			return errm.New("unexpected", "state", newState)
		}
		return nil
	}

	user.setState(bundle.State, msgID)

	return nil
}

func (b *Bot) sendError(userID int64, msg string, opts ...any) {
	user := b.GetUser(userID).(*userContextImpl)
	if user == nil {
		b.bot.log.Error("failed to send error message", "user_id", userID, "error", errEmptyUserID)
		return
	}
	if user.Messages().ErrorID != 0 {
		b.bot.delete(user.ID(), user.Messages().ErrorID)
	}
	closeBtn := b.msgs.Messages(user.Language()).CloseBtn()
	if closeBtn != "" {
		_, unique := getBtnIDAndUnique(closeBtn)
		btn := tele.Btn{
			Unique: unique,
			Text:   closeBtn,
		}
		opts = append(opts, SingleRow(btn))
		b.bot.handle(b, func(ctx tele.Context) error {
			if user.Messages().ErrorID == 0 {
				return nil
			}
			b.bot.delete(user.ID(), user.Messages().ErrorID)
			user.setErrorMessage(0)
			return nil
		})
	}
	msgID, err := b.bot.send(user.ID(), msg, append(opts, tele.Silent)...)
	if err != nil {
		b.bot.log.Error("failed to send error message", "user_id", user.ID(), "error", err)
		return
	}
	user.setErrorMessage(msgID)
}

var (
	errEmptyUserID = errm.New("empty user id")
	errEmptyMsgID  = errm.New("empty msg id")
)

type baseBot struct {
	tbot *tele.Bot
	log  Logger

	defaultOptions []any
	middlewares    []func(upd *tele.Update) bool
}

func newBaseBot(token string, opts Options) (*baseBot, error) {
	b := &baseBot{
		log:            opts.Logger,
		defaultOptions: []any{opts.Config.ParseMode},
		middlewares:    make([]func(upd *tele.Update) bool, 0),
	}

	if opts.Config.NoPreview {
		b.defaultOptions = append(b.defaultOptions, tele.NoPreview)
	}

	var poller tele.Poller
	poller = &tele.LongPoller{Timeout: opts.Config.LPTimeout}
	if opts.Poller != nil {
		poller = opts.Poller
	}

	bot, err := tele.NewBot(tele.Settings{
		Token:   token,
		Poller:  tele.NewMiddlewarePoller(poller, b.middleware),
		Client:  &http.Client{Timeout: 2 * opts.Config.LPTimeout},
		Verbose: opts.Config.Debug,
		OnError: func(err error, ctx tele.Context) {
			var userID int64
			if ctx != nil && ctx.Chat() != nil {
				userID = ctx.Chat().ID
			}
			b.log.Error("error callback", "error", err, "user_id", userID)
		},
		Offline: opts.Config.TestMode,
	})
	if err != nil {
		return nil, errm.Wrap(err, "new telebot")
	}
	b.tbot = bot

	return b, nil
}

func (b *baseBot) start() {
	b.log.Info("bot is starting")
	lang.Go(b.log, b.tbot.Start)
}

func (b *baseBot) stop() {
	b.log.Info("bot is stopping")
	b.tbot.Stop()
}

func (b *baseBot) addMiddleware(f func(upd *tele.Update) bool) {
	b.middlewares = append(b.middlewares, f)
}

func (b *baseBot) handle(endpoint any, handler tele.HandlerFunc) {
	b.tbot.Handle(endpoint, handler)
}

func (b *baseBot) send(userID int64, msg string, options ...any) (int, error) {
	if userID == 0 {
		return 0, errEmptyUserID
	}

	m, err := b.tbot.Send(userIDWrapper(userID), msg, append(options, b.defaultOptions...)...)
	if err != nil {
		return 0, err
	}

	return m.ID, nil
}

func (b *baseBot) edit(userID int64, msgID int, what any, options ...any) error {
	if userID == 0 {
		return errEmptyUserID
	}
	if msgID == 0 {
		return errEmptyMsgID
	}

	_, err := b.tbot.Edit(getEditable(userID, msgID), what, append(options, b.defaultOptions...)...)
	if err != nil {
		if strings.Contains(err.Error(), "message is not modified") {
			b.log.Debug("message is not modified", "msg_id", msgID, "user_id", userID)
			return nil
		}
		return err
	}

	return nil
}

func (b *baseBot) editReplyMarkup(userID int64, msgID int, markup *tele.ReplyMarkup) error {
	if userID == 0 {
		return errEmptyUserID
	}
	if msgID == 0 {
		return errEmptyMsgID
	}

	_, err := b.tbot.EditReplyMarkup(getEditable(userID, msgID), markup)
	if err != nil {
		return err
	}

	return nil
}

func (b *baseBot) delete(userID int64, msgIDs ...int) error {
	if userID == 0 {
		return errEmptyUserID
	}

	errSet := errm.NewList()

	for _, msgID := range msgIDs {
		if msgID == 0 {
			return errEmptyMsgID
		}
		if err := b.tbot.Delete(getEditable(userID, msgID)); err != nil {
			errSet.Add(err)
		}
	}

	return errSet.Err()
}

func (b *baseBot) deleteHistory(userID int64, lastMessageID int) map[int]struct{} {
	deleted := map[int]struct{}{}
	var counter int
	for msgID := lastMessageID; msgID > 1; msgID-- {
		err := b.delete(userID, msgID)
		if err != nil {
			counter += 1
		} else {
			counter = 0
			deleted[msgID] = struct{}{}
		}
		if counter == 5 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	return deleted
}

func (b *baseBot) middleware(upd *tele.Update) bool {
	if upd.MyChatMember != nil {
		if lang.Deref(upd.MyChatMember.NewChatMember).Role == "kicked" {
			b.log.Warn("bot is blocked",
				"user_id", lang.Deref(upd.MyChatMember.Sender).ID,
				"username", lang.Deref(upd.MyChatMember.Sender).Username,
				"old_role", lang.Deref(upd.MyChatMember.OldChatMember).Role,
				"new_role", lang.Deref(upd.MyChatMember.NewChatMember).Role)

			return false
		}
		if lang.Deref(upd.MyChatMember.OldChatMember).Role == "kicked" {
			b.log.Info("bot is unblocked",
				"user_id", lang.Deref(upd.MyChatMember.Sender).ID,
				"username", lang.Deref(upd.MyChatMember.Sender).Username,
				"old_role", lang.Deref(upd.MyChatMember.OldChatMember).Role,
				"new_role", lang.Deref(upd.MyChatMember.NewChatMember).Role)

			return false
		}
	}

	for _, m := range b.middlewares {
		if !m(upd) {
			return false
		}
	}

	return true
}

type userIDWrapper int64

func (u userIDWrapper) Recipient() string {
	return strconv.Itoa(int(u))
}

type noopLogger struct{}

func (noopLogger) Debug(msg string, fields ...any) {}
func (noopLogger) Info(msg string, fields ...any)  {}
func (noopLogger) Warn(msg string, fields ...any)  {}
func (noopLogger) Error(msg string, fields ...any) {}

type updateLogger struct {
	l Logger
}

func (r *updateLogger) Log(t UpdateType, fields ...any) {
	r.l.Debug(t.String(), fields...)
}

func getEditable(senderID int64, messageID int) tele.Editable {
	return &tele.Message{ID: messageID, Chat: &tele.Chat{ID: senderID}}
}

func maxLen(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func getSender(upd *tele.Update) *tele.User {
	switch {
	case upd.Callback != nil:
		return upd.Callback.Sender
	case upd.Message != nil:
		return upd.Message.Sender
	case upd.Query != nil:
		return upd.Query.Sender
	case upd.MessageReaction != nil:
		return upd.MessageReaction.User
	case upd.InlineResult != nil:
		return upd.InlineResult.Sender
	case upd.MyChatMember != nil:
		return upd.MyChatMember.Sender
	case upd.EditedMessage != nil:
		return upd.EditedMessage.Sender
	case upd.ShippingQuery != nil:
		return upd.ShippingQuery.Sender
	case upd.ChannelPost != nil:
		return upd.ChannelPost.Sender
	case upd.EditedChannelPost != nil:
		return upd.EditedChannelPost.Sender
	case upd.PreCheckoutQuery != nil:
		return upd.PreCheckoutQuery.Sender
	case upd.PollAnswer != nil:
		return upd.PollAnswer.Sender
	case upd.ChatJoinRequest != nil:
		return upd.ChatJoinRequest.Sender
	case upd.BusinessMessage != nil:
		return upd.BusinessMessage.Sender
	case upd.BusinessConnection != nil:
		return upd.BusinessConnection.Sender
	case upd.EditedBusinessMessage != nil:
		return upd.EditedBusinessMessage.Sender
	default:
		return nil
	}
}
