# Bote: Interactive Telegram Bot Framework for Go

Bote is a powerful wrapper for Telebot.v4 that simplifies building interactive Telegram bots with smart message management, user state tracking, and advanced keyboard handling.

[![Go Reference](https://pkg.go.dev/badge/github.com/maxbolgarin/bote.svg)](https://pkg.go.dev/github.com/maxbolgarin/bote)
[![Go Report Card](https://goreportcard.com/badge/github.com/maxbolgarin/bote)](https://goreportcard.com/report/github.com/maxbolgarin/bote)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Features

- **Smart Message Management**: Main, head, notification, and history message handling
- **User State Tracking**: Track and manage user states across different messages
- **Interactive Keyboards**: Easy creation and management of inline keyboards
- **Middleware Support**: Add custom middleware functions
- **Internationalization**: Built-in support for multiple languages
- **Persistence**: Optional user data persistence between bot restarts
- **Context-based API**: Clean, context-based API for handlers

## Installation

```bash
go get -u github.com/maxbolgarin/bote
```

## Concepts

Bote introduces several important concepts that make building interactive bots easier:

### Message Types

- **Main Message**: The primary interactive message shown to the user
- **Head Message**: Optional message displayed above the main message
- **Notification Message**: Temporary messages for user notifications
- **Error Message**: Special messages for error handling
- **History Messages**: Previous main messages that have been replaced

### User States

Each message in Bote has an associated state, allowing you to track and control the flow of your bot's interaction with users. States help you manage user interactions across different messages and sessions.

### Context

The `Context` interface provides access to user information, message management, and keyboard creation. All handlers receive a Context object to interact with the bot and user.

## Quick Start

```go
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/maxbolgarin/bote"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		panic("TELEGRAM_BOT_TOKEN is not set")
	}

	// Create a new bot with configuration
	cfg := bote.Config{
		DefaultLanguageCode: "en",
		NoPreview:           true,
	}

	b, err := bote.New(ctx, token, bote.WithConfig(cfg))
	if err != nil {
		panic(err)
	}

	// Start the bot with a start handler
	b.Start(ctx, startHandler, nil)

	// Wait for interrupt signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch

	// Stop the bot gracefully
	b.Stop()
}

// Handler for the /start command
func startHandler(ctx bote.Context) error {
	// Create an inline keyboard
	kb := bote.InlineBuilder(3, bote.OneBytePerRune,
		ctx.Btn("Option 1", option1Handler),
		ctx.Btn("Option 2", option2Handler),
		ctx.Btn("Option 3", option3Handler),
	)
	
	// Send a main message with the keyboard
	return ctx.SendMain(bote.NoChange, "Welcome to my bot! Choose an option:", kb)
}

func option1Handler(ctx bote.Context) error {
	return ctx.SendMain(bote.NoChange, "You selected Option 1", nil)
}

func option2Handler(ctx bote.Context) error {
	return ctx.SendMain(bote.NoChange, "You selected Option 2", nil)
}

func option3Handler(ctx bote.Context) error {
	return ctx.SendMain(bote.NoChange, "You selected Option 3", nil)
}
```

## Core Concepts in Detail

### Bot Initialization

Create a new bot instance with the `New` function:

```go
b, err := bote.New(ctx, token, bote.WithConfig(cfg))
```

Available options:
- `WithConfig(cfg Config)`: Set bot configuration
- `WithUserDB(db UsersStorage)`: Use a custom user storage
- `WithLogger(log Logger)`: Use a custom logger
- `WithMessages(msgs MessageProvider)`: Use custom message provider
- `WithUpdateLogger(l UpdateLogger)`: Use custom update logger

### Webhook Configuration

Bote supports webhooks for receiving updates from Telegram, which can be more efficient than long polling in some environments.

To configure webhooks, you need to set the following fields in the `bote.Config` struct or use their corresponding environment variables:

- **`WebhookURL`** (`string`): The publicly accessible HTTPS URL for your webhook. Telegram will send updates to this URL.
  - Environment variable: `BOTE_WEBHOOK_URL`
- **`ListenAddress`** (`string`): The IP address and port the bot will listen on for incoming webhook requests (e.g., `:8443` or `0.0.0.0:8080`).
  - Environment variable: `BOTE_LISTEN_ADDRESS`
  - Defaults to `:8443` if `WebhookURL` is set and `ListenAddress` is not.
- **`TLSKeyFile`** (`string`, optional): Path to your TLS private key file. Required if you want the bot to handle HTTPS directly.
  - Environment variable: `BOTE_TLS_KEY_FILE`
- **`TLSCertFile`** (`string`, optional): Path to your TLS public certificate file. Required if you want the bot to handle HTTPS directly.
  - Environment variable: `BOTE_TLS_CERT_FILE`

**When to use TLS Key/Cert files:**

- If your bot is directly exposed to the internet and you want it to handle HTTPS encryption itself, provide both `TLSKeyFile` and `TLSCertFile`.
- If your bot is behind a reverse proxy (like Nginx or Caddy) that handles HTTPS termination, you typically don't need to set `TLSKeyFile` and `TLSCertFile` in the bot's configuration. The reverse proxy would forward plain HTTP traffic to the `ListenAddress` of the bot.

Make sure your `WebhookURL` is correctly pointing to where your bot is listening, and that your firewall/network configuration allows Telegram servers to reach your `ListenAddress`.

### States

States in Bote track the user's progress and context. Create custom states like this:

```go
type AppState string

func (s AppState) String() string { return string(s) }
func (s AppState) IsText() bool   { return false }
func (s AppState) NotChanged() bool { return s == "" }

const (
	StateStart    AppState = "start"
	StateMainMenu AppState = "main_menu"
	StateSettings AppState = "settings"
	StateProfile  AppState = "profile"
	// Define text expecting states
	StateAwaitingName AppState = "awaiting_name"
)

// For text-expecting states, override IsText
func (s AppState) IsText() bool {
	return s == StateAwaitingName
}
```

### Message Management

Bote provides several methods for managing messages:

```go
// Send a main message
ctx.SendMain(newState, "Hello, world!", keyboard)

// Send a notification
ctx.SendNotification("Notification message", nil)

// Edit the main message
ctx.EditMain(newState, "Updated message", newKeyboard)

// Send both main and head messages
ctx.Send(newState, "Main message", "Head message", mainKeyboard, headKeyboard)
```

### User Management

Access and manage user data:

```go
// Get the current user
user := ctx.User()

// Access user properties
userID := user.ID()
username := user.Username()
language := user.Language()

// Get user state
currentState := user.StateMain()

// Get all messages
messages := user.Messages()
```

### Keyboard Creation

Create interactive inline keyboards:

```go
// Simple inline keyboard with buttons in one row
keyboard := bote.SingleRow(
    ctx.Btn("Button 1", handler1),
    ctx.Btn("Button 2", handler2),
)

// Multi-row keyboard with automatic layout
keyboard := bote.InlineBuilder(2, bote.TwoBytesPerRune,
    ctx.Btn("Button 1", handler1),
    ctx.Btn("Button 2", handler2),
    ctx.Btn("Button 3", handler3),
    ctx.Btn("Button 4", handler4),
)

// Manual keyboard building
kb := bote.NewKeyboard(2) // 2 buttons per row maximum
kb.Add(ctx.Btn("Button 1", handler1))
kb.Add(ctx.Btn("Button 2", handler2))
kb.StartNewRow() // Force new row
kb.Add(ctx.Btn("Button 3", handler3))
keyboard := kb.CreateInlineMarkup()
```

### Handling Button Callbacks

When creating buttons, you register handlers that will be called when the button is pressed:

```go
ctx.Btn("Settings", func(ctx bote.Context) error {
    // This will be called when the Settings button is pressed
    
    // You can access button data
    data := ctx.Data() // Get complete callback data
    
    // Create a new keyboard for settings
    kb := bote.InlineBuilder(1, bote.OneBytePerRune,
        ctx.Btn("Profile", profileHandler),
        ctx.Btn("Language", languageHandler),
        ctx.Btn("Back", mainMenuHandler),
    )
    
    return ctx.EditMain(StateSettings, "Settings", kb)
})
```

### Handling Text Messages

Handle text messages by setting a text handler and checking the user state:

```go
// Set the text handler
b.SetTextHandler(func(ctx bote.Context) error {
    // Get the user state
    state := ctx.User().StateMain()
    
    // Handle based on state
    switch state {
    case StateAwaitingName:
        name := ctx.Text()
        // Process the name
        return ctx.SendMain(StateMainMenu, "Thank you, " + name + "!", mainMenuKeyboard)
    default:
        return ctx.SendMain(StateMainMenu, "I don't understand that command.", mainMenuKeyboard)
    }
})

// In another handler, set the state to await text
func askNameHandler(ctx bote.Context) error {
    return ctx.SendMain(StateAwaitingName, "Please enter your name:", nil)
}
```

## Advanced Features

### Persistence

Implement the `UsersStorage` interface to persist user data between bot restarts:

```go
type MyStorage struct {
    // Your storage implementation
}

func (s *MyStorage) Insert(ctx context.Context, userModel bote.UserModel) error {
    // Insert user into database
}

func (s *MyStorage) Find(ctx context.Context, id int64) (bote.UserModel, bool, error) {
    // Find user in database
}

func (s *MyStorage) Update(id int64, userModel *bote.UserModelDiff) {
    // Update user in database asynchronously
}

// Use your storage when creating the bot
storage := &MyStorage{}
b, err := bote.New(ctx, token, bote.WithUserDB(storage))
```

### Custom Message Provider

Implement the `MessageProvider` interface for custom messaging and internationalization:

```go
type MyMessages struct{}

func (m *MyMessages) Messages(languageCode string) bote.Messages {
    switch languageCode {
    case "ru":
        return &MyRussianMessages{}
    default:
        return &MyEnglishMessages{}
    }
}

type MyEnglishMessages struct{}

func (m *MyEnglishMessages) GeneralError() string {
    return "An error occurred"
}

func (m *MyEnglishMessages) FatalError() string {
    return "A fatal error occurred! Please restart with /start"
}

func (m *MyEnglishMessages) PrepareMessage(msg string, u bote.User, newState bote.State, msgID int, isHistorical bool) string {
    // Customize message as needed
    return msg
}

// Use your message provider when creating the bot
msgs := &MyMessages{}
b, err := bote.New(ctx, token, bote.WithMessages(msgs))
```

### Middleware

Add custom middleware to process updates before they reach handlers:

```go
b.AddMiddleware(func(upd *tele.Update, user bote.User) bool {
    // Process update
    // Return false to stop update processing
    return true
})
```

### Bot Restart Recovery

Handle bot restarts by providing a state map to the `Start` method:

```go
stateMap := map[bote.State]bote.InitBundle{
    StateMainMenu: {
        Handler: mainMenuHandler,
    },
    StateSettings: {
        Handler: settingsHandler,
    },
}

b.Start(ctx, startHandler, stateMap)
```

## Complete Example: Multi-step Form Bot

Here's a more complete example of a bot that guides users through a form:

```go
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/maxbolgarin/bote"
)

// Define custom states
type AppState string

func (s AppState) String() string { return string(s) }
func (s AppState) IsText() bool   { return s == StateAwaitingName || s == StateAwaitingEmail || s == StateAwaitingAge }
func (s AppState) NotChanged() bool { return s == "" }

const (
	StateStart        AppState = "start"
	StateMainMenu     AppState = "main_menu"
	StateForm         AppState = "form"
	StateFormComplete AppState = "form_complete"
	
	// Text states
	StateAwaitingName  AppState = "awaiting_name"
	StateAwaitingEmail AppState = "awaiting_email"
	StateAwaitingAge   AppState = "awaiting_age"
)

// User data
type UserData struct {
	Name  string
	Email string
	Age   string
}

var userData = make(map[int64]UserData)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		panic("TELEGRAM_BOT_TOKEN is not set")
	}

	cfg := bote.Config{
		DefaultLanguageCode: "en",
		NoPreview:           true,
	}

	b, err := bote.New(ctx, token, bote.WithConfig(cfg))
	if err != nil {
		panic(err)
	}

	// Set text handler
	b.SetTextHandler(textHandler)

	// Define state map for bot restart
	stateMap := map[bote.State]bote.InitBundle{
		StateMainMenu: {
			Handler: mainMenuHandler,
		},
		StateForm: {
			Handler: formHandler,
		},
		StateAwaitingName: {
			Handler: askNameHandler,
		},
		StateAwaitingEmail: {
			Handler: askEmailHandler,
		},
		StateAwaitingAge: {
			Handler: askAgeHandler,
		},
		StateFormComplete: {
			Handler: formCompleteHandler,
		},
	}

	// Start the bot
	b.Start(ctx, startHandler, stateMap)

	// Wait for interrupt signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch

	// Stop the bot gracefully
	b.Stop()
}

// Start handler - initial command
func startHandler(ctx bote.Context) error {
	kb := bote.InlineBuilder(1, bote.OneBytePerRune,
		ctx.Btn("Main Menu", mainMenuHandler),
	)
	return ctx.SendMain(StateStart, "Welcome to the Form Bot!", kb)
}

// Main menu handler
func mainMenuHandler(ctx bote.Context) error {
	kb := bote.InlineBuilder(1, bote.OneBytePerRune,
		ctx.Btn("Fill Form", formHandler),
	)
	return ctx.SendMain(StateMainMenu, "Main Menu", kb)
}

// Form handler - starts the form process
func formHandler(ctx bote.Context) error {
	// Initialize user data
	userData[ctx.User().ID()] = UserData{}
	
	// Start with name
	return askNameHandler(ctx)
}

// Ask for name
func askNameHandler(ctx bote.Context) error {
	return ctx.SendMain(StateAwaitingName, "Please enter your name:", nil)
}

// Ask for email
func askEmailHandler(ctx bote.Context) error {
	return ctx.SendMain(StateAwaitingEmail, "Please enter your email:", nil)
}

// Ask for age
func askAgeHandler(ctx bote.Context) error {
	return ctx.SendMain(StateAwaitingAge, "Please enter your age:", nil)
}

// Form complete
func formCompleteHandler(ctx bote.Context) error {
	user := userData[ctx.User().ID()]
	
	message := bote.NewBuilder()
	message.Writeln("Form Complete!")
	message.Writeln("")
	message.Writeln("Your information:")
	message.Writeln("Name: " + user.Name)
	message.Writeln("Email: " + user.Email)
	message.Writeln("Age: " + user.Age)
	
	kb := bote.InlineBuilder(1, bote.OneBytePerRune,
		ctx.Btn("Main Menu", mainMenuHandler),
		ctx.Btn("Fill Again", formHandler),
	)
	
	return ctx.SendMain(StateFormComplete, message.String(), kb)
}

// Handle text messages based on state
func textHandler(ctx bote.Context) error {
	state := ctx.User().StateMain()
	text := ctx.Text()
	userID := ctx.User().ID()
	
	switch state {
	case StateAwaitingName:
		userData[userID] = UserData{Name: text}
		return askEmailHandler(ctx)
		
	case StateAwaitingEmail:
		user := userData[userID]
		user.Email = text
		userData[userID] = user
		return askAgeHandler(ctx)
		
	case StateAwaitingAge:
		user := userData[userID]
		user.Age = text
		userData[userID] = user
		return formCompleteHandler(ctx)
		
	default:
		return ctx.SendNotification("I don't understand that command.", nil)
	}
}
```

## Best Practices

1. **Organize states**: Keep your states organized and well-defined
2. **Use context methods**: Rely on Context methods for message management
3. **Plan your message flow**: Design how messages will flow and states will change
4. **Implement persistence**: Use a database to store user data between restarts
5. **Use middlewares**: Add middlewares for logging, analytics, or rate limiting
6. **Error handling**: Always handle errors in your handlers

## License

MIT
