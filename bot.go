package bote

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/maxbolgarin/abstract"
	"github.com/maxbolgarin/erro"
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
}

// Bot is a main struct of this package. It contains all necessary components for working with Telegram bot.
type Bot struct {
	bot  *baseBot
	um   *userManagerImpl
	msgs MessageProvider
	rlog UpdateLogger

	defaultLanguage Language

	middlewares        *abstract.SafeSlice[MiddlewareFunc]
	stateMap           *abstract.SafeMap[string, InitBundle]
	startHandler       HandlerFunc
	deleteMessages     bool
	logUpdates         bool
	hideUserDataInLogs bool

	wp          *webhookPoller
	webhookInit chan struct{}
}

// New creates the bot with optional options.
// It starts the bot in a separate goroutine.
// You should call [Bot.Stop] to gracefully shutdown the bot.
func New(ctx context.Context, token string, optsFuncs ...func(*Options)) (*Bot, error) {
	var opts Options
	for _, f := range optsFuncs {
		f(&opts)
	}
	return NewWithOptions(ctx, token, opts)
}

// NewWithOptions starts the bot with options.
// It starts the bot in a separate goroutine.
// You should call [Bot.Stop] to gracefully shutdown the bot.
func NewWithOptions(ctx context.Context, token string, opts Options) (*Bot, error) {
	if token == "" {
		return nil, erro.New("token cannot be empty")
	}
	opts, err := prepareOpts(opts)
	if err != nil {
		return nil, erro.Wrap(err, "prepare opts")
	}

	b, err := newBaseBot(ctx, token, opts)
	if err != nil {
		return nil, erro.Wrap(err, "start bot")
	}

	um, err := newUserManager(opts)
	if err != nil {
		return nil, erro.Wrap(err, "new user manager")
	}

	bote := &Bot{
		bot:  b,
		um:   um,
		msgs: opts.Msgs,
		rlog: opts.UpdateLogger,

		defaultLanguage:    opts.Config.Bot.DefaultLanguage,
		middlewares:        abstract.NewSafeSlice[MiddlewareFunc](),
		stateMap:           abstract.NewSafeMap[string, InitBundle](),
		deleteMessages:     lang.Deref(opts.Config.Bot.DeleteMessages),
		logUpdates:         lang.Deref(opts.Config.Log.LogUpdates),
		webhookInit:        make(chan struct{}),
		hideUserDataInLogs: opts.Config.Log.HideUserData,
	}

	b.addMiddleware(bote.masterMiddleware, tele.ChatPrivate)
	bote.AddUserMiddleware(bote.cleanMiddleware)
	bote.AddUserMiddleware(bote.startMiddleware)

	// To trigger initUserHandler on callback (there is no registered handlers if user is not inited)
	bote.Handle(tele.OnCallback, bote.emptyHandler)

	if wp, ok := opts.Poller.(*webhookPoller); ok {
		bote.wp = wp
		bote.webhookInit = wp.stopCh
	}

	return bote, nil
}

// Start starts the bot in a separate goroutine.
// StartHandler is a handler for /start command.
// StateMap is a map for initing users after bot restart.
// It runs an assigned handler for every active user message when user makes a request by message's state.
// Inline buttons will trigger onCallback handler if you don't init them after bot restart.
// You can pass nil map if you don't need to reinit messages.
func (b *Bot) Start(ctx context.Context, startHandler HandlerFunc, stateMap map[State]InitBundle) chan struct{} {
	b.startHandler = startHandler
	for k, v := range stateMap {
		b.stateMap.Set(k.String(), v)
	}
	b.Handle(startCommand, func(ctx Context) (err error) {
		defer func() {
			// Delete here to prevent from automatic double /start sending in telegram client
			if ctx.Text() == startCommand {
				if err = b.bot.delete(ctx.User().ID(), ctx.MessageID()); err != nil {
					b.bot.log.Debug("failed to delete user message",
						"user_id", prepareUserID(ctx.User().ID(), b.um.priv),
						"msg_id", ctx.MessageID(),
						"error", err.Error(),
					)
				}
			}
		}()
		return b.startHandler(ctx)
	})

	b.bot.log.Info("bot is starting")

	stopChannel := make(chan struct{})
	botStopSignal := b.bot.tbot.GetStopSignal()

	lang.Go(b.bot.log, func() {

		// Start bot
		lang.Go(b.bot.log, b.bot.tbot.Start)

		// Wait for
		//  1. Outer context stopping
		//  2. Webhook init error
		//  3. Telebot stoped poller
		select {
		case <-ctx.Done():
		case <-b.webhookInit:
		case <-botStopSignal:
		}

		b.bot.log.Info("bot is stopping")

		// Stop bot poller and updates processing
		b.bot.tbot.Stop()

		// Shutdown webhook server
		if b.wp != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := b.wp.shutdown(ctx); err != nil {
				b.bot.log.Error("failed to shutdown webhook poller", "error", err.Error())
			}
		}

		// Send a signal that bot is stopped
		close(stopChannel)
	})

	return stopChannel
}

// Bot returns the underlying *tele.Bot.
func (b *Bot) Bot() *tele.Bot {
	return b.bot.tbot
}

// GetAllUsersFromCache returns all loaded users.
func (b *Bot) GetAllUsersFromCache() []User {
	return b.um.getAllUsers()
}

// SendInChat sends a message to a specific chat ID and thread ID.
// chatID is the target chat ID, threadID is the target thread ID (0 for no thread).
// msg is the message to send.
// kb is the keyboard to send.
// opts are additional options for sending the message.
func (b *Bot) SendInChat(chatID int64, threadID int, msg string, kb *tele.ReplyMarkup, opts ...any) (int, error) {
	if chatID == 0 {
		b.bot.log.Error("chat ID cannot be empty", "chat_id", chatID, "thread_id", threadID)
		b.bot.metr.incError(MetricsErrorBadUsage, MetricsErrorSeveritHigh)
		return 0, nil
	}
	if msg == "" {
		b.bot.log.Error("message cannot be empty", "chat_id", chatID, "thread_id", threadID)
		b.bot.metr.incError(MetricsErrorBadUsage, MetricsErrorSeveritHigh)
		return 0, nil
	}

	if threadID > 0 {
		opts = append(opts, tele.MessageThreadID(threadID))
	}

	return b.bot.send(chatID, msg, append(opts, kb)...)
}

func (b *Bot) EditInChat(chatID int64, msgID int, msg string, kb *tele.ReplyMarkup, opts ...any) error {
	if chatID == 0 {
		b.bot.log.Error("chat ID cannot be empty", "chat_id", chatID, "msg_id", msgID)
		b.bot.metr.incError(MetricsErrorBadUsage, MetricsErrorSeveritHigh)
		return nil
	}
	if msgID == 0 {
		b.bot.log.Error("message ID cannot be empty", "chat_id", chatID, "msg_id", msgID)
		b.bot.metr.incError(MetricsErrorBadUsage, MetricsErrorSeveritHigh)
		return nil
	}
	if msg == "" {
		b.bot.log.Error("message cannot be empty", "chat_id", chatID, "msg_id", msgID)
		b.bot.metr.incError(MetricsErrorBadUsage, MetricsErrorSeveritHigh)
		return nil
	}

	return b.bot.edit(chatID, msgID, msg, append(opts, kb)...)
}

func (b *Bot) DeleteInChat(chatID int64, msgID int) error {
	if chatID == 0 {
		b.bot.log.Error("chat ID cannot be empty", "chat_id", chatID, "msg_id", msgID)
		b.bot.metr.incError(MetricsErrorBadUsage, MetricsErrorSeveritHigh)
		return nil
	}
	if msgID == 0 {
		b.bot.log.Error("message ID cannot be empty", "chat_id", chatID, "msg_id", msgID)
		b.bot.metr.incError(MetricsErrorBadUsage, MetricsErrorSeveritHigh)
		return nil
	}

	return b.bot.delete(chatID, msgID)
}

// CreateUserFromModel creates a new user from [UserModel].
// If addToCache is true, the user will be added to the cache.
// It is useful when you want to preinit user in cache before he makes a request.
// User should have plain ID to be added to the cache.
func (b *Bot) CreateUserFromModel(model UserModel, addToCache bool) User {
	user := b.um.newUserContext(model, b.um.priv)
	id := user.ID()
	if addToCache && id != 0 {
		if ok := b.um.users.set(id, user); !ok {
			b.bot.log.Warn("failed to add user to cache", "user_id", prepareUserID(id, b.um.priv))
		}
	}
	return user
}

// AddUserMiddleware adds middleware functions that will be called on each update for a user.
func (b *Bot) AddUserMiddleware(f ...MiddlewareFunc) {
	b.middlewares.Append(f...)
}

var allChatTypes = []tele.ChatType{tele.ChatPrivate, tele.ChatGroup, tele.ChatSuperGroup, tele.ChatChannel, tele.ChatChannelPrivate}

// AddMiddleware adds middleware functions that will be called on each update for a specific chat type.
func (b *Bot) AddMiddleware(f MiddlewareFuncTele, chatType ...tele.ChatType) {
	b.bot.addMiddleware(f, chatType...)
}

// Handle sets handler for any endpoint. Endpoint can be string or callback button.
func (b *Bot) Handle(endpoint any, f HandlerFunc) {
	b.bot.handle(endpoint, func(c tele.Context) (err error) {
		// Measure handler execution time
		start := time.Now()

		ctx := b.newContext(c)
		defer func() {
			if b.logUpdates {
				upd := c.Update()
				b.logUpdate(&upd, ctx.user)
			}

			b.bot.metr.recordHandlerFinish()
			duration := time.Since(start)
			b.bot.metr.observeHandlerDuration(duration)
			ctx.user.clearUserID()
		}()

		defer lang.RecoverWithErrAndStack(b.bot.log, &err)

		// If chat is private run user flow
		if ctx.user != nil {
			msgID := ctx.MessageID()
			if !ctx.user.isMsgInited(msgID) && b.stateMap.Len() > 0 {
				if err = b.initUserHandler(ctx, msgID); err != nil {
					return ctx.handleError(err)
				}
			}

			if ep, ok := endpoint.(string); ok && ep == tele.OnText {
				lastMsg := ctx.user.lastTextMessage()
				ctx.textMsgID = lastMsg

				// /start was already handled
				if ctx.ct.Text() == startCommand {
					return nil
				}
			}
		}

		if err = f(ctx); err != nil {
			return ctx.handleError(err)
		}

		if c.Callback() != nil {
			if err = c.Respond(&tele.CallbackResponse{}); err != nil {
				b.bot.log.Debug("failed to respond to callback", "error", err.Error())
				b.bot.metr.incError(MetricsErrorTelegramAPI, MetricsErrorSeverityLow)
			}
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
	msgs := ctx.user.Messages()
	defer func() {
		ctx.user.setMsgInited(msgID)
		if msgID == msgs.HeadID {
			ctx.user.setMsgInited(msgs.MainID)
		}
		if msgID == msgs.MainID && msgs.HeadID != 0 {
			ctx.user.setMsgInited(msgs.HeadID)
		}
	}()

	if msgs.MainID == 0 || ctx.user.StateMain() == FirstRequest {
		return nil
	}

	if msgID == msgs.NotificationID || msgID == msgs.ErrorID {
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
		b.bot.log.Warn("button handler not found", "user_id", prepareUserID(ctx.user.ID(), b.um.priv), "button_id", btnID)
		b.bot.metr.incError(MetricsErrorInvalidUserState, MetricsErrorSeveritHigh)
		return nil
	}

	upd := tele.Update{
		Callback: &tele.Callback{
			Sender:  &tele.User{ID: ctx.user.ID()}, // preserve sender
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

func (*Bot) emptyHandler(Context) error {
	return nil
}

func (b *Bot) masterMiddleware(upd *tele.Update) bool {
	defer lang.Recover(b.bot.log)

	sender := getSender(upd)
	if sender == nil {
		b.bot.log.Error(fmt.Sprintf("cannot get sender from update: %+v", upd))
		b.bot.metr.incError(MetricsErrorInternal, MetricsErrorSeveritHigh)
		return false
	}

	user, err := b.um.prepareUser(sender)
	if err != nil {
		b.bot.log.Error("cannot prepare user", "error", err.Error(),
			"user_id", prepareUserID(sender.ID, b.um.priv),
			"username", sender.Username,
		)
		b.sendError(sender.ID, b.msgs.Messages(b.defaultLanguage).GeneralError())
		b.bot.metr.incError(MetricsErrorInternal, MetricsErrorSeveritHigh)
		return false
	}
	user.setUserID(sender.ID)
	b.bot.metr.addActiveUser(user.ID())

	if st, ok := user.State(getMsgID(upd)); ok {
		b.bot.metr.incStateRequest(st)
	}

	return b.middlewares.Range(func(mf MiddlewareFunc) bool {
		return mf(upd, user)
	})
}

func (b *Bot) cleanMiddleware(upd *tele.Update, userRaw User) bool {
	user, ok := userRaw.(*userContextImpl)
	if !ok {
		b.bot.log.Error("failed to cast user to userContextImpl",
			"user_id", prepareUserID(userRaw.ID(), b.um.priv),
		)
		b.bot.metr.incError(MetricsErrorInternal, MetricsErrorSeveritHigh)
		return false
	}

	msgIDs := user.Messages()
	if msgIDs.ErrorID > 0 {
		err := b.bot.delete(user.ID(), msgIDs.ErrorID)
		if err != nil {
			b.bot.log.Debug("failed to delete error message",
				"user_id", user.user.ID.String(),
				"msg_id", msgIDs.ErrorID,
				"error", err.Error(),
			)
		}
		user.setErrorMessage(0)
	}
	if upd.Message != nil && b.deleteMessages {
		if upd.Message.Text == startCommand {
			return true
		}
		err := b.bot.delete(user.ID(), upd.Message.ID)
		if err != nil {
			b.bot.log.Debug("failed to delete user message",
				"user_id", user.user.ID.String(),
				"msg_id", upd.Message.ID,
				"error", err.Error(),
			)
		}
	}

	if upd.Callback != nil {
		if match := cbackRx.FindAllStringSubmatch(upd.Callback.Data, -1); match != nil {
			user.setBtnAndPayload(getNameFromUnique(match[0][1]), match[0][3])
		}
	}

	// Sanitize incoming message text to prevent injection attacks
	if upd.Message != nil && upd.Message.Text != "" {
		upd.Message.Text = sanitizeText(upd.Message.Text, 1e5)
	}

	return true
}

var cbackRx = regexp.MustCompile(`([-\w]+)(\|(.+))?`)

func (b *Bot) logUpdate(upd *tele.Update, user *userContextImpl) bool {
	if user == nil {
		fields := make([]any, 0, 8)
		chatID, chatType, ok := getChatID(upd)
		if ok {
			fields = append(fields, "chat_id", chatID, "chat_type", chatType)
		}
		sender := getSender(upd)
		if sender != nil {
			fields = append(fields, "user_id", prepareUserID(sender.ID, b.um.priv))
			if !b.um.priv.IsStrict() {
				fields = append(fields, "username", sender.Username)
			}
		}
		b.rlog.Log(NotPrivateUpdate, fields...)
		return true
	}

	fields := make([]any, 0, 14)
	fields = append(fields,
		"user_id", user.user.ID.String(),
	)
	if !b.um.priv.IsStrict() {
		fields = append(fields, "username", user.user.Info.Username)
	}

	switch {
	case upd.Message != nil:
		fields = append(fields, "state", user.user.State.Main, "msg_id", upd.Message.ID)
		if !b.hideUserDataInLogs {
			fields = append(fields, "text", maxLen(upd.Message.Text, MaxTextLenInLogs))
		}
		if msgID, st := user.lastTextMessageState(); msgID != 0 {
			fields = append(fields, "text_state", st, "text_state_msg_id", msgID)
		}
		b.rlog.Log(MessageUpdate, fields...)

	case upd.Callback != nil:
		if upd.Callback.Message != nil {
			st, ok := user.State(upd.Callback.Message.ID)
			if !ok || st.NotChanged() {
				st = user.StateMain()
			}
			fields = append(fields, "state", st, "msg_id", upd.Callback.Message.ID)
		} else {
			fields = append(fields, "state", user.user.State.Main)
		}

		if !b.hideUserDataInLogs {
			btnName, payload := user.getBtnAndPayload()
			if btnName != "" {
				fields = append(fields, "button", btnName)
			}
			if payload != "" {
				fields = append(fields, "payload", payload)
			}
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

	if userRaw.StateMain() == FirstRequest && b.startHandler != nil {
		err := b.startHandler(b.newContextFromUpdate(*upd))
		if err != nil {
			b.bot.log.Error("failed to handle start command", "error", err.Error())
			b.bot.metr.incError(MetricsErrorHandler, MetricsErrorSeveritHigh)
		}
	}
	return true
}

func (b *Bot) userFields(user User, fields ...any) []any {
	f := make([]any, 0, len(fields)+6)
	if u, ok := user.(*userContextImpl); ok {
		// No mutex lock for the price of type assertion
		f = append(f, "user_id", u.user.ID.String())
		if !b.um.priv.IsStrict() {
			f = append(f, "username", u.user.Info.Username)
		}
		f = append(f, "state", u.user.State.Main)
	} else {
		f = append(f, "user_id", prepareUserID(user.ID(), b.um.priv))
		if !b.um.priv.IsStrict() {
			f = append(f, "username", user.Username())
		}
		f = append(f, "state", user.StateMain())
	}
	return append(f, fields...)
}

func (b *Bot) initUserMsg(user *userContextImpl, msgID int) error {
	msgs := user.Messages()
	if msgID == msgs.HeadID {
		b.bot.log.Debug("got head message in init, set as main",
			"user_id", user.user.ID.String(),
			"msg_id", msgID,
		)
		msgID = msgs.MainID
	}

	st, ok := user.State(msgID)
	if !ok {
		b.bot.log.Warn("forget history message without state",
			"user_id", user.user.ID.String(),
			"msg_id", msgID,
		)
		user.forgetHistoryMessage(msgID)
		b.bot.metr.incError(MetricsErrorInvalidUserState, MetricsErrorSeverityLow)
		return nil
	}

	bundle, foundBundle := b.stateMap.Lookup(st.String())
	if !foundBundle {
		b.bot.log.Warn("init bundle not found",
			"user_id", user.user.ID.String(),
			"msg_id", msgID, "state", st)
		b.bot.metr.incError(MetricsErrorInvalidUserState, MetricsErrorSeverityLow)
		bundle = InitBundle{
			Handler: b.startHandler,
		}
	}

	b.bot.log.Debug("init user message",
		"user_id", user.user.ID.String(),
		"msg_id", msgID, "state", st)

	targetBundleErr := b.init(bundle, user, msgID, st)
	if targetBundleErr != nil {
		if !foundBundle {
			// got error by startHandler
			return erro.Wrap(targetBundleErr, "start handler",
				"user_id", user.user.ID.String(),
				"msg_id", msgID, "state", st)
		}
		startHandlerErr := b.init(InitBundle{
			Handler: b.startHandler,
		}, user, msgID, Unknown)
		if startHandlerErr != nil {
			return erro.Wrap(startHandlerErr, "start handler",
				"user_id", user.user.ID.String(),
				"msg_id", msgID, "state", st, "first_error", targetBundleErr)
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
	msgs := user.Messages()
	mainBefore := msgs.MainID
	headBefore := msgs.HeadID

	err := bundle.Handler(&contextImpl{
		bt:   b,
		ct:   b.bot.tbot.NewContext(upd),
		user: user,
	})
	if err != nil {
		if strings.Contains(err.Error(), "is not modified") {
			b.bot.log.Debug("message is not modified in init",
				"user_id", user.user.ID.String(),
				"msg_id", msgID,
			)
			return nil
		}
		return erro.Wrap(err, "run handler")
	}

	msgs = user.Messages()
	newMainID := msgs.MainID
	newHeadID := msgs.HeadID

	if mainBefore != newMainID {
		b.bot.log.Debug("main message id changed in init",
			"user_id", user.user.ID.String(),
			"msg_id", msgID,
		)
		if err := b.bot.delete(user.ID(), mainBefore); err != nil {
			b.bot.log.Error("error deleting old main message",
				"user_id", user.user.ID.String(),
				"msg_id", mainBefore,
				"error", err.Error(),
			)
		}
		user.forgetHistoryMessage(mainBefore)
		msgID = newMainID
	}
	if headBefore != newHeadID {
		b.bot.log.Debug("head message id changed in init",
			"user_id", user.user.ID.String(),
			"msg_id", msgID,
		)
		if err := b.bot.delete(user.ID(), headBefore); err != nil {
			b.bot.log.Error("error deleting old head message",
				"user_id", user.user.ID.String(),
				"msg_id", headBefore,
				"error", err.Error(),
			)
		}
		user.forgetHistoryMessage(headBefore)
	}

	newState, ok := user.State(msgID)
	if !ok {
		return erro.New("new state not found")
	}

	// Expected state not changed after running handler
	if expectedState != Unknown {
		// TODO: is it OK to ignore unexpected state?
		// if newState != expectedState {
		// 	return erro.New("unexpected", "state", newState)
		// }
		return nil
	}

	b.bot.log.Warn("init to start handler",
		"user_id", user.user.ID.String(),
		"msg_id", msgID,
		"state", newState,
	)

	return nil
}

func (b *Bot) sendError(userID int64, msg string, opts ...any) {
	user := b.um.getUser(userID)
	if user == nil {
		b.bot.log.Error("failed to send error message", "user_id", prepareUserID(userID, b.um.priv), "error", errEmptyUserID)
		b.bot.metr.incError(MetricsErrorInternal, MetricsErrorSeveritHigh)
		return
	}
	msgs := user.Messages()
	if msgID := msgs.ErrorID; msgID != 0 {
		err := b.bot.delete(user.ID(), msgID)
		if err != nil {
			b.bot.log.Debug("failed to delete error message", "user_id", prepareUserID(userID, b.um.priv), "msg_id", msgID, "error", err.Error())
		}
	}
	closeBtn := b.msgs.Messages(user.Language()).CloseBtn()
	if closeBtn != "" {
		_, unique := getBtnIDAndUnique(closeBtn)
		btn := tele.Btn{
			Unique: unique,
			Text:   closeBtn,
		}
		opts = append(opts, SingleRow(btn))
		b.bot.handle(&btn, func(tele.Context) error {
			msgs := user.Messages()
			if msgs.ErrorID == 0 {
				return nil
			}
			err := b.bot.delete(user.ID(), msgs.ErrorID)
			if err != nil {
				b.bot.log.Error("failed to delete error message using close button",
					"user_id", prepareUserID(userID, b.um.priv),
					"msg_id", msgs.ErrorID,
					"error", err.Error())
			}
			user.setErrorMessage(0)
			return nil
		})
	}
	msgID, err := b.bot.send(user.ID(), msg, append(opts, tele.Silent)...)
	if err != nil {
		b.bot.log.Error("failed to send error message", "user_id", prepareUserID(userID, b.um.priv), "error", err.Error())
		return
	}
	user.setErrorMessage(msgID)
}

var (
	errEmptyUserID = erro.New("empty user id")
	errEmptyMsgID  = erro.New("empty msg id")
)

type baseBot struct {
	tbot *tele.Bot
	metr *metrics
	log  Logger

	priv           PrivacyMode
	defaultOptions []any
	middlewares    map[tele.ChatType][]func(upd *tele.Update) bool
}

func newBaseBot(ctx context.Context, token string, opts Options) (*baseBot, error) {
	b := &baseBot{
		metr:           opts.metrics,
		log:            opts.Logger,
		priv:           opts.Config.Bot.Privacy.Mode,
		defaultOptions: []any{opts.Config.Bot.ParseMode},
		middlewares:    make(map[tele.ChatType][]func(upd *tele.Update) bool),
	}

	if opts.Config.Bot.NoPreview {
		b.defaultOptions = append(b.defaultOptions, tele.NoPreview)
	}

	bot, err := tele.NewBot(ctx, tele.Settings{
		Token:  token,
		Poller: tele.NewMiddlewarePoller(opts.Poller, b.middleware),
		Client: &http.Client{Timeout: 2 * opts.Config.LongPolling.Timeout},
		OnError: func(err error, ctx tele.Context) {
			var userID int64
			if ctx != nil && ctx.Chat() != nil {
				userID = ctx.Chat().ID
			}
			b.log.Error("error callback", "error", err.Error(), "user_id", prepareUserID(userID, b.priv))
			b.metr.incError(MetricsErrorHandler, MetricsErrorSeveritHigh)
		},
		Updates: defaultUpdatesChannelCapacity,
		Verbose: opts.Config.Log.DebugIncomingUpdates,
		Offline: opts.Offline,
	})
	if err != nil {
		return nil, erro.Wrap(err, "new telebot")
	}
	b.tbot = bot

	return b, nil
}

func (b *baseBot) addMiddleware(f MiddlewareFuncTele, chatType ...tele.ChatType) {
	if len(chatType) == 0 {
		chatType = allChatTypes
	}
	for _, ct := range chatType {
		b.middlewares[ct] = append(b.middlewares[ct], f)
	}
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
		b.metr.incError(MetricsErrorTelegramAPI, MetricsErrorSeverityLow)
		return 0, err
	}

	b.metr.incSendMessagesTotal()

	return m.ID, nil
}

func (b *baseBot) sendFile(userID int64, file []byte, name string, options ...any) (int, error) {
	if userID == 0 {
		return 0, errEmptyUserID
	}

	doc := &tele.Document{
		File:     tele.FromReader(bytes.NewReader(file)),
		FileName: name,
	}

	m, err := b.tbot.Send(userIDWrapper(userID), doc, append(options, b.defaultOptions...)...)
	if err != nil {
		b.metr.incError(MetricsErrorTelegramAPI, MetricsErrorSeverityLow)
		return 0, err
	}

	b.metr.incSendMessagesTotal()

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
			b.log.Debug("message is not modified", "msg_id", msgID, "user_id", prepareUserID(userID, b.priv))
			return nil
		}
		b.metr.incError(MetricsErrorTelegramAPI, MetricsErrorSeverityLow)
		return err
	}

	b.metr.incEditMessagesTotal()

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
		b.metr.incError(MetricsErrorTelegramAPI, MetricsErrorSeverityLow)
		return err
	}

	b.metr.incEditMessagesTotal()

	return nil
}

func (b *baseBot) delete(userID int64, msgIDs ...int) error {
	if userID == 0 {
		return errEmptyUserID
	}

	errSet := erro.NewList()

	for _, msgID := range msgIDs {
		if msgID == 0 {
			errSet.Add(errEmptyMsgID)
			continue
		}
		if err := b.tbot.Delete(getEditable(userID, msgID)); err != nil {
			b.metr.incError(MetricsErrorTelegramAPI, MetricsErrorSeverityLow)
			errSet.Add(err)
			continue
		}
		b.metr.incDeleteMessagesTotal()
	}

	return errSet.Err()
}

func (b *baseBot) deleteHistory(userID int64, lastMessageID int) map[int]struct{} {
	deleted := map[int]struct{}{}
	var counter int
	for msgID := lastMessageID; msgID > 1; msgID-- {
		err := b.delete(userID, msgID)
		if err != nil {
			counter++
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
	b.metr.incUpdate()

	if upd.MyChatMember != nil {
		if lang.Deref(upd.MyChatMember.NewChatMember).Role == "kicked" {
			b.log.Warn("bot is blocked",
				"user_id", prepareUserID(lang.Deref(upd.MyChatMember.Sender).ID, b.priv),
				"old_role", lang.Deref(upd.MyChatMember.OldChatMember).Role,
				"new_role", lang.Deref(upd.MyChatMember.NewChatMember).Role)
			b.metr.incError(MetricsErrorBotBlocked, MetricsErrorSeverityLow)

			return false
		}
		if lang.Deref(upd.MyChatMember.OldChatMember).Role == "kicked" {
			b.log.Info("bot is unblocked",
				"user_id", prepareUserID(lang.Deref(upd.MyChatMember.Sender).ID, b.priv),
				"old_role", lang.Deref(upd.MyChatMember.OldChatMember).Role,
				"new_role", lang.Deref(upd.MyChatMember.NewChatMember).Role)
			b.metr.incError(MetricsErrorBotBlocked, MetricsErrorSeverityLow)

			return false
		}
	}

	_, chatType, ok := getChatID(upd)
	if !ok {
		b.log.Error("cannot get chat ID from update", "update", upd)
		return false
	}

	middlewares, ok := b.middlewares[chatType]
	if !ok || len(middlewares) == 0 {
		return true
	}
	for _, m := range middlewares {
		if !m(upd) {
			return false
		}
	}

	return true
}

func getChatID(upd *tele.Update) (int64, tele.ChatType, bool) {
	switch {
	case upd.Message != nil:
		if upd.Message.Chat != nil {
			return upd.Message.Chat.ID, upd.Message.Chat.Type, true
		}
	case upd.EditedMessage != nil:
		if upd.EditedMessage.Chat != nil {
			return upd.EditedMessage.Chat.ID, upd.EditedMessage.Chat.Type, true
		}
	case upd.ChannelPost != nil:
		if upd.ChannelPost.Chat != nil {
			return upd.ChannelPost.Chat.ID, upd.ChannelPost.Chat.Type, true
		}
	case upd.EditedChannelPost != nil:
		if upd.EditedChannelPost.Chat != nil {
			return upd.EditedChannelPost.Chat.ID, upd.EditedChannelPost.Chat.Type, true
		}
	case upd.MessageReaction != nil:
		if upd.MessageReaction.Chat != nil {
			return upd.MessageReaction.Chat.ID, upd.MessageReaction.Chat.Type, true
		}
	case upd.MessageReactionCount != nil:
		if upd.MessageReactionCount.Chat != nil {
			return upd.MessageReactionCount.Chat.ID, upd.MessageReactionCount.Chat.Type, true
		}
	case upd.Callback != nil:
		if upd.Callback.Message != nil && upd.Callback.Message.Chat != nil {
			return upd.Callback.Message.Chat.ID, upd.Callback.Message.Chat.Type, true
		}
	case upd.MyChatMember != nil:
		if upd.MyChatMember.Chat != nil {
			return upd.MyChatMember.Chat.ID, upd.MyChatMember.Chat.Type, true
		}
	case upd.ChatMember != nil:
		if upd.ChatMember.Chat != nil {
			return upd.ChatMember.Chat.ID, upd.ChatMember.Chat.Type, true
		}
	case upd.ChatJoinRequest != nil:
		if upd.ChatJoinRequest.Chat != nil {
			return upd.ChatJoinRequest.Chat.ID, upd.ChatJoinRequest.Chat.Type, true
		}
	case upd.Boost != nil:
		if upd.Boost.Chat != nil {
			return upd.Boost.Chat.ID, upd.Boost.Chat.Type, true
		}
	case upd.BoostRemoved != nil:
		if upd.BoostRemoved.Chat != nil {
			return upd.BoostRemoved.Chat.ID, upd.BoostRemoved.Chat.Type, true
		}
	case upd.BusinessConnection != nil:
		return upd.BusinessConnection.UserChatID, tele.ChatPrivate, true

	case upd.BusinessMessage != nil:
		if upd.BusinessMessage.Chat != nil {
			return upd.BusinessMessage.Chat.ID, upd.BusinessMessage.Chat.Type, true
		}
	case upd.EditedBusinessMessage != nil:
		if upd.EditedBusinessMessage.Chat != nil {
			return upd.EditedBusinessMessage.Chat.ID, upd.EditedBusinessMessage.Chat.Type, true
		}
	case upd.DeletedBusinessMessages != nil:
		if upd.DeletedBusinessMessages.Chat != nil {
			return upd.DeletedBusinessMessages.Chat.ID, upd.DeletedBusinessMessages.Chat.Type, true
		}
	}
	return 0, "", false
}

type userIDWrapper int64

func (u userIDWrapper) Recipient() string {
	return strconv.Itoa(int(u))
}

type noopLogger struct{}

func (noopLogger) Debug(string, ...any) {}
func (noopLogger) Info(string, ...any)  {}
func (noopLogger) Warn(string, ...any)  {}
func (noopLogger) Error(string, ...any) {}

type updateLogger struct {
	l Logger
}

func (r *updateLogger) Log(t UpdateType, fields ...any) {
	r.l.Debug(t.String(), fields...)
}

func getEditable(senderID int64, messageID int) tele.Editable {
	return &tele.Message{ID: messageID, Chat: &tele.Chat{ID: senderID}}
}

func maxLen(s string, mx int) string {
	if len(s) <= mx {
		return s
	}
	return s[:mx]
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

func getMsgID(upd *tele.Update) int {
	switch {
	case upd.Callback != nil:
		return upd.Callback.Message.ID
	case upd.Message != nil:
		return upd.Message.ID
	case upd.EditedMessage != nil:
		return upd.EditedMessage.ID
	case upd.ChannelPost != nil:
		return upd.ChannelPost.ID
	case upd.EditedChannelPost != nil:
		return upd.EditedChannelPost.ID
	case upd.MessageReaction != nil:
		return upd.MessageReaction.MessageID
	case upd.BusinessMessage != nil:
		return upd.BusinessMessage.ID
	case upd.EditedBusinessMessage != nil:
		return upd.EditedBusinessMessage.ID
	default:
		return 0
	}
}
