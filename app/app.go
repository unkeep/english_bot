package app

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/unkeep/english_bot/db"
	"github.com/unkeep/english_bot/tg"
)

type App struct{}

func (app *App) Run(ctx context.Context) error {
	log.Println("Run")
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("getConfig: %w", err)
	}

	// herocu param
	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	httpServer := http.Server{
		Addr:    "0.0.0.0:" + port,
		Handler: http.HandlerFunc(healthcheckHandler),
	}

	// nolint: errcheck
	go httpServer.ListenAndServe()
	go func() {
		<-ctx.Done()
		_ = httpServer.Shutdown(context.Background())
	}()

	log.Println("GetRepo")
	repo, err := db.GetRepo(ctx, cfg.MongoURI)
	if err != nil {
		return fmt.Errorf("db.GetRepo: %w", err)
	}

	log.Println("GetBot")
	tgBot, err := tg.GetBot(cfg.TgToken)
	if err != nil {
		return fmt.Errorf("tg.GetBot: %w", err)
	}

	msgChan := make(chan tg.UserMsg)
	btnClickChan := make(chan tg.BtnClick)
	critErrosChan := make(chan error)

	go func() {
		if err := tgBot.GetUpdates(ctx, msgChan, btnClickChan); err != nil {
			critErrosChan <- fmt.Errorf("tgBot.GetUpdates: %w", err)
		}
	}()

	h := handler{
		cfg:   cfg,
		repo:  repo,
		tgBot: tgBot,
	}

	hh := func(name string, param interface{}, f func(ctx context.Context) error) {
		ctx, cancel := context.WithTimeout(ctx, time.Second*10)
		defer cancel()
		if err := f(ctx); err != nil {
			log.Printf("%s(%+v): %s\n", name, param, err.Error())
			_, err = tgBot.SendMessage(tg.BotMessage{
				ChatID: cfg.TgAdminChatID,
				Text: fmt.Sprintf("⚠️ handler: %s, error:\n```%s```\ncontext:\n```%+v```\n",
					name, err.Error(), param),
				TextMarkdown: true,
			})
			if err != nil {
				log.Println(err.Error())
			}
		}
	}

	log.Println("selecting channels")
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-critErrosChan:
			return err
		case msg := <-msgChan:
			hh("handleUserMessage", msg, func(ctx context.Context) error {
				return h.handleUserMessage(ctx, msg)
			})
		case click := <-btnClickChan:
			hh("handleBtnClick", click, func(ctx context.Context) error {
				return h.handleBtnClickMessage(ctx, click)
			})
		}
	}
}
