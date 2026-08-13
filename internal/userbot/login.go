package userbot

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"

	"github.com/grepfruitx/instgobot/internal/config"
)

type terminalAuth struct{}

func prompt(label string) (string, error) {
	fmt.Print(label)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (terminalAuth) Phone(context.Context) (string, error) {
	return prompt("Phone number: ")
}

func (terminalAuth) Password(context.Context) (string, error) {
	return prompt("2FA password (if any): ")
}

func (terminalAuth) Code(context.Context, *tg.AuthSentCode) (string, error) {
	return prompt("Code: ")
}

func (terminalAuth) AcceptTermsOfService(context.Context, tg.HelpTermsOfService) error {
	return nil
}

func (terminalAuth) SignUp(context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, fmt.Errorf("sign-up not supported — this must be an existing Telegram account")
}

func Login(ctx context.Context, cfg *config.Config) error {
	c := NewClient(cfg)
	return c.tg.Run(ctx, func(ctx context.Context) error {
		flow := auth.NewFlow(terminalAuth{}, auth.SendCodeOptions{})
		return flow.Run(ctx, c.tg.Auth())
	})
}
