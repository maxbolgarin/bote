# Bote: Interactive Telegram Bot Framework for Go

Bote is a powerful wrapper for [Telebot v4](https://gopkg.in/telebot.v4) that simplifies building interactive Telegram bots with smart message management, user state tracking, and advanced keyboard handling.

[![Go Version][version-img]][doc] [![GoDoc][doc-img]][doc] [![Build][ci-img]][ci] [![Coverage][coverage-img]][coverage] [![GoReport][report-img]][report]

## Features

- **Smart Message Management** — main, head, notification, error, and history message lifecycle
- **User State Tracking** — per-message states with text input handling
- **Interactive Keyboards** — inline keyboard builder with automatic row layout
- **Middleware Support** — user-level and chat-type middlewares
- **Internationalization** — built-in multi-language message provider
- **Privacy & Encryption** — optional strict mode with AES-256 encrypted user IDs
- **Persistence** — pluggable storage with ordered async writes via [gorder](https://github.com/maxbolgarin/gorder)
- **Webhook Support** — built-in webhook server with TLS, secret token, IP filtering, and rate limiting
- **Prometheus Metrics** — updates, handlers, errors, active users, session length, webhooks
- **Bot Restart Recovery** — automatic re-initialization of user messages via state map
- **Context-based API** — clean handler interface with `Context` for all operations

## Installation

```bash
go get github.com/maxbolgarin/bote
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/maxbolgarin/bote"
)

func main() {
    token := os.Getenv("TELEGRAM_BOT_TOKEN")
    if token == "" {
        log.Fatalln("TELEGRAM_BOT_TOKEN is not set")
    }

    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    b, err := bote.New(ctx, token)
    if err != nil {
        log.Fatalln(err)
    }

    stopCh := b.Start(ctx, startHandler, nil)
    <-stopCh
}

func startHandler(ctx bote.Context) error {
    kb := bote.InlineBuilder(3, bote.OneBytePerRune,
        ctx.Btn("Option 1", option1Handler),
        ctx.Btn("Option 2", option2Handler),
        ctx.Btn("Option 3", option3Handler),
    )
    return ctx.SendMain(bote.NoChange, "Welcome! Choose an option:", kb)
}

func option1Handler(ctx bote.Context) error {
    return ctx.EditMain(bote.NoChange, "You selected Option 1", nil)
}

func option2Handler(ctx bote.Context) error {
    return ctx.EditMain(bote.NoChange, "You selected Option 2", nil)
}

func option3Handler(ctx bote.Context) error {
    return ctx.EditMain(bote.NoChange, "You selected Option 3", nil)
}
```

## Core Concepts

### Message Types

Bote manages five message types per user, each with automatic lifecycle handling:

| Type | Description | Behavior |
|------|-------------|----------|
| **Main** | Primary interactive message | Previous main becomes history on new send |
| **Head** | Optional message above main | Deleted when new main is sent |
| **Notification** | Temporary user notification | Old notification auto-deleted on new one |
| **Error** | Error feedback | Auto-deleted on next user action |
| **History** | Previous main messages | Tracked for editing and cleanup |

### States

Each message has an associated state. States control which handler runs when a user interacts with an old message after a bot restart.

```go
// Define your states as a string type implementing the State interface
type AppState string

func (s AppState) String() string  { return string(s) }
func (s AppState) IsText() bool    { return s == StateAwaitingInput }
func (s AppState) NotChanged() bool { return false }

const (
    StateMenu          AppState = "menu"
    StateSettings      AppState = "settings"
    StateAwaitingInput AppState = "awaiting_input" // IsText() returns true
)
```

Built-in states: `bote.NoChange`, `bote.FirstRequest`, `bote.Unknown`, `bote.Disabled`.

### Context

Every handler receives a `Context` that provides:

```go
func myHandler(ctx bote.Context) error {
    // User info
    ctx.User().ID()
    ctx.User().Username()
    ctx.User().Language()
    ctx.User().StateMain()

    // Message operations
    ctx.SendMain(state, "text", keyboard)
    ctx.EditMain(state, "text", keyboard)
    ctx.Send(state, "main text", "head text", mainKb, headKb)
    ctx.SendNotification("info", nil)
    ctx.SendError("something went wrong")

    // Callback data
    ctx.ButtonID()
    ctx.Data()           // raw: "a|b|c"
    ctx.DataParsed()     // []string{"a", "b", "c"}

    // Text input
    ctx.Text()

    // Custom values (persisted)
    ctx.User().SetValue("key", value)
    val, ok := ctx.User().GetValue("key")

    return nil
}
```

## Configuration

### Using Option Functions

```go
b, err := bote.New(ctx, token,
    bote.WithDefaultLanguage("en"),
    bote.WithLogger(myLogger),
    bote.WithUserDB(myStorage),
    bote.WithMsgsProvider(myMessages),
    bote.WithDebugIncomingUpdates(),
)
```

### Using Config Struct

```go
b, err := bote.New(ctx, token, func(opts *bote.Options) {
    opts.Config = bote.Config{
        Bot: bote.BotConfig{
            DefaultLanguage: "en",
            DeleteMessages:  bote.Ptr(true),
            NoPreview:       true,
        },
        Log: bote.LogConfig{
            Enable:     bote.Ptr(true),
            LogUpdates: bote.Ptr(true),
        },
    }
    opts.UserDB = myStorage
    opts.Msgs = myMessages
})
```

### Environment Variables

All config fields can be set via `BOTE_*` environment variables (e.g., `BOTE_DEFAULT_LANGUAGE=en`).

## Keyboards

```go
// Single row
kb := bote.SingleRow(
    ctx.Btn("Yes", yesHandler),
    ctx.Btn("No", noHandler),
)

// Auto-layout with column count and rune type
kb := bote.InlineBuilder(2, bote.TwoBytesPerRune,
    ctx.Btn("Option A", handlerA),
    ctx.Btn("Option B", handlerB),
    ctx.Btn("Option C", handlerC),
    ctx.Btn("Option D", handlerD),
)

// Manual builder
kb := bote.NewKeyboard(3)
kb.Add(ctx.Btn("One", h1))
kb.Add(ctx.Btn("Two", h2))
kb.StartNewRow()
kb.Add(ctx.Btn("Three", h3))
kb.AddFooter(ctx.Btn("Back", backHandler))
markup := kb.CreateInlineMarkup()

// Buttons with callback data
ctx.Btn("Delete", deleteHandler, userID, itemID)
// In handler: ctx.DataParsed() returns []string{userID, itemID}
```

Rune size types for automatic row sizing: `OneBytePerRune` (English), `TwoBytesPerRune` (Cyrillic), `FourBytesPerRune` (emoji).

## Text Input Handling

Register text-expecting states and set a text handler:

```go
// In your state definition
func (s AppState) IsText() bool {
    return s == StateAwaitingName || s == StateAwaitingEmail
}

// Set the text handler
b.SetTextHandler(func(ctx bote.Context) error {
    text := ctx.Text()

    switch ctx.User().StateMain() {
    case StateAwaitingName:
        ctx.User().SetValue("name", text)
        return ctx.EditMain(StateAwaitingEmail, "Now enter your email:", nil)

    case StateAwaitingEmail:
        ctx.User().SetValue("email", text)
        name, _ := ctx.User().GetValue("name")
        msg := bote.FB("Name: ") + name.(string) + "\n" + bote.FB("Email: ") + text
        return ctx.EditMain(StateMenu, msg, menuKeyboard(ctx))

    default:
        return ctx.SendNotification("Send /start to begin", nil)
    }
})

// Trigger text input by sending a message with a text state
func askNameHandler(ctx bote.Context) error {
    return ctx.SendMain(StateAwaitingName, "Enter your name:", nil)
}
```

## Persistence

Implement `UsersStorage` to persist user data between restarts:

```go
type MyStorage struct {
    db *sql.DB
}

func (s *MyStorage) Insert(ctx context.Context, user bote.UserModel) error {
    // Insert user into database
    return nil
}

func (s *MyStorage) Find(ctx context.Context, id bote.FullUserID) (bote.UserModel, bool, error) {
    // Find user by ID (use id.IDPlain or id.IDHMAC depending on privacy mode)
    return bote.UserModel{}, false, nil
}

func (s *MyStorage) UpdateAsync(id bote.FullUserID, diff *bote.UserModelDiff) {
    // Apply partial update. Bote wraps this with an ordered queue (gorder),
    // so updates are guaranteed to arrive in order per user.
    // You can use a simple synchronous DB call here.
}

b, err := bote.New(ctx, token, bote.WithUserDB(&MyStorage{db: db}))
```

## Bot Restart Recovery

Provide a state map so users can continue from where they left off:

```go
stateMap := map[bote.State]bote.InitBundle{
    StateMenu: {
        Handler: menuHandler,
    },
    StateSettings: {
        Handler: settingsHandler,
    },
    StateAwaitingName: {
        Handler: askNameHandler,
    },
}

stopCh := b.Start(ctx, startHandler, stateMap)
```

When a user clicks a button on an old message after a restart, bote looks up the message's state in this map and runs the corresponding handler to rebuild the UI.

## Middleware

```go
// User-level middleware (private chats only)
b.AddUserMiddleware(func(upd *tele.Update, user bote.User) bool {
    log.Printf("User %d made an action", user.ID())
    return true // return false to drop the update
})

// Chat-type middleware
b.AddMiddleware(func(upd *tele.Update) bool {
    // Rate limiting, analytics, etc.
    return true
}, tele.ChatGroup, tele.ChatSuperGroup)
```

## Webhook Mode

```go
b, err := bote.New(ctx, token,
    bote.WithWebhook("https://example.com/webhook", ":8443"),
    bote.WithWebhookSecretToken("my-secret"),
    bote.WithWebhookRateLimit(100, 200),
    bote.WithWebhookSecurityHeaders(),
    bote.WithWebhookAllowedTelegramIPs(),
)
```

Options: `WithWebhookCertificate`, `WithWebhookGenerateCertificate`, `WithWebhookAllowedIPs`, `WithWebhookMetrics`.

## Privacy Mode

Strict privacy mode encrypts user IDs with AES-256 and stores only HMAC for lookups:

```go
encKey := "hex-encoded-32-byte-key"
hmacKey := "hex-encoded-32-byte-key"

b, err := bote.New(ctx, token,
    bote.WithStrictPrivacyMode(&encKey, nil, &hmacKey, nil),
)
```

In strict mode: no usernames or names are stored, user IDs are encrypted in the database, and logs show only HMAC prefixes.

## Prometheus Metrics

```go
import "github.com/prometheus/client_golang/prometheus"

registry := prometheus.NewRegistry()
b, err := bote.New(ctx, token,
    bote.WithMetricsConfig(bote.MetricsConfig{
        Registry: registry,
    }),
)
```

Tracked metrics: `bote_updates_total`, `bote_handlers_in_flight`, `bote_handler_duration_seconds`, `bote_errors_total`, `bote_messages_send_total`, `bote_users_current_active`, `bote_users_session_length_seconds`, and webhook metrics.

## Message Formatting

```go
msg := bote.FB("Bold") + " and " + bote.FI("italic") + "\n"
msg += bote.FC("code") + " or " + bote.FP("pre", "go") + "\n"
msg += bote.FS("strikethrough") + " " + bote.FU("underline")

// Using the builder
b := bote.NewBuilder()
b.Writeln(bote.FB("User Profile"))
b.Writeln("")
b.Writeln("Name: " + name)
b.Writeln("Email: " + email)
b.WriteIf(isAdmin, bote.FB("Admin"))
msg = b.String()
```

`FB` = bold, `FI` = italic, `FC` = code, `FP` = pre, `FS` = strikethrough, `FU` = underline.

## Complete Example: Todo Bot

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/signal"
    "strconv"
    "syscall"

    "github.com/maxbolgarin/bote"
)

type State string

func (s State) String() string  { return string(s) }
func (s State) IsText() bool    { return s == StateAddingTask }
func (s State) NotChanged() bool { return false }

const (
    StateMenu       State = "menu"
    StateAddingTask State = "adding_task"
    StateViewTasks  State = "view_tasks"
)

func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    b, err := bote.New(ctx, os.Getenv("TELEGRAM_BOT_TOKEN"))
    if err != nil {
        log.Fatalln(err)
    }

    b.SetTextHandler(textHandler)

    stopCh := b.Start(ctx, menuHandler, map[bote.State]bote.InitBundle{
        StateMenu:       {Handler: menuHandler},
        StateAddingTask: {Handler: addTaskHandler},
        StateViewTasks:  {Handler: viewTasksHandler},
    })
    <-stopCh
}

func menuHandler(ctx bote.Context) error {
    kb := bote.InlineBuilder(1, bote.OneBytePerRune,
        ctx.Btn("Add Task", addTaskHandler),
        ctx.Btn("View Tasks", viewTasksHandler),
    )
    return ctx.SendMain(StateMenu, bote.FB("Todo List")+"\nChoose an action:", kb)
}

func addTaskHandler(ctx bote.Context) error {
    kb := bote.SingleRow(ctx.Btn("Cancel", menuHandler))
    return ctx.EditMain(StateAddingTask, "Enter your task:", kb)
}

func viewTasksHandler(ctx bote.Context) error {
    tasks, ok := ctx.User().GetValue("tasks")
    if !ok || len(tasks.([]string)) == 0 {
        kb := bote.SingleRow(ctx.Btn("Add Task", addTaskHandler))
        return ctx.EditMain(StateViewTasks, "No tasks yet!", kb)
    }

    b := bote.NewBuilder()
    b.Writeln(bote.FB("Your Tasks:"))
    b.Writeln("")
    for i, task := range tasks.([]string) {
        b.Writeln(fmt.Sprintf("%d. %s", i+1, task))
    }

    kb := bote.InlineBuilder(1, bote.OneBytePerRune,
        ctx.Btn("Add Task", addTaskHandler),
        ctx.Btn("Clear All", func(ctx bote.Context) error {
            ctx.User().DeleteValue("tasks")
            return viewTasksHandler(ctx)
        }),
        ctx.Btn("Back", menuHandler),
    )
    return ctx.EditMain(StateViewTasks, b.String(), kb)
}

func textHandler(ctx bote.Context) error {
    if ctx.User().StateMain() != StateAddingTask {
        return nil
    }
    task := ctx.Text()

    tasks, ok := ctx.User().GetValue("tasks")
    var list []string
    if ok {
        list = tasks.([]string)
    }
    list = append(list, task)
    ctx.User().SetValue("tasks", list)

    ctx.SendNotification("Task added: "+bote.FI(task), nil)
    return viewTasksHandler(ctx)
}
```

## Public Chat Support

Bote can handle messages in groups and channels alongside private chats:

```go
// Register a handler for text messages in any chat
b.Handle(tele.OnText, func(ctx bote.Context) error {
    if !ctx.IsPrivate() {
        // Group/channel message
        if ctx.IsMentioned() {
            return ctx.SendInChat(ctx.ChatID(), 0, "Hello from the bot!", nil)
        }
        return nil
    }
    // Private message — handled by text handler
    return nil
})
```

## API Reference

See the full API documentation on [pkg.go.dev](https://pkg.go.dev/github.com/maxbolgarin/bote).

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

---

[version-img]: https://img.shields.io/badge/Go-%3E%3D%201.24-%23007d9c
[doc-img]: https://pkg.go.dev/badge/github.com/maxbolgarin/bote
[doc]: https://pkg.go.dev/github.com/maxbolgarin/bote
[ci-img]: https://github.com/maxbolgarin/bote/actions/workflows/go.yml/badge.svg
[ci]: https://github.com/maxbolgarin/bote/actions
[report-img]: https://goreportcard.com/badge/github.com/maxbolgarin/bote
[report]: https://goreportcard.com/report/github.com/maxbolgarin/bote
[coverage-img]: https://codecov.io/gh/maxbolgarin/bote/branch/main/graph/badge.svg
[coverage]: https://codecov.io/gh/maxbolgarin/bote/branch/main
