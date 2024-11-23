package bote

import (
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/maxbolgarin/errm"
	"github.com/maxbolgarin/lang"
	"github.com/maxbolgarin/logze"
	tele "gopkg.in/telebot.v3"
)

const (
	DefaultLPTimeout = 15 * time.Second
)

var (
	EmptyUserIDErr = errm.New("empty user id")
	EmptyMsgIDErr  = errm.New("empty msg id")
)

type Config struct {
	Token     string        `yaml:"token" env:"BOTE_TOKEN"`
	LPTimeout time.Duration `yaml:"lp_timeout" env:"BOTE_TIMEOUT"`

	ParseMode tele.ParseMode `yaml:"mode" env:"BOTE_PARSE_MODE"`
	NoPreview bool           `yaml:"no_preview" env:"BOTE_NO_PREVIEW"`
	Debug     bool           `yaml:"debug" env:"BOTE_DEBUG"`

	Logger logze.Logger `yaml:"-" env:"-"`
}

func (cfg Config) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.Token, validation.Required),
		validation.Field(&cfg.ParseMode, validation.In(tele.ModeHTML, tele.ModeMarkdown, tele.ModeMarkdownV2)),
		validation.Field(&cfg.LPTimeout, validation.Min(time.Second)),
	)
}

func (cfg Config) prepare() (Config, error) {
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return cfg, errm.Wrap(err, "read env")
	}

	if err := cfg.Validate(); err != nil {
		return cfg, errm.Wrap(err, "validate")
	}

	cfg.ParseMode = lang.Check(cfg.ParseMode, tele.ModeHTML)
	cfg.LPTimeout = lang.Check(cfg.LPTimeout, DefaultLPTimeout)
	cfg.Logger = lang.If(cfg.Logger.NotInited(), logze.Log, cfg.Logger)

	return cfg, nil
}
