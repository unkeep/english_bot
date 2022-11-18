package app

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/v4"

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

	words, err := repo.Words.GetAll(ctx)
	if err != nil {
		log.Fatalf("GetAll: %s", err.Error())
	}

	log.Println("got words", len(words))

	if connStr := os.Getenv("PG_DB"); connStr != "" {
		const chatID = 114969818
		conn, err := pgx.Connect(context.Background(), connStr)
		if err != nil {
			log.Fatalf("sql.Open: %s", err.Error())
		}

		pingCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		if err := conn.Ping(pingCtx); err != nil {
			log.Fatalf("ping: %s", err.Error())
		}

		for _, w := range words {
			query := `insert into words (
	chat_id,
	id,
	key_text,
	hint_text,
	hint_file_id,
	added_at,
	last_touched_at,
	touched_count,
	success_count,
	fail_count,
	success_pct,
	ready
)
values($1,$2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

			id, _ := uuid.NewV4()
			_, err := conn.Exec(ctx, query, chatID, id.String(), w.Text, w.Hint, w.HintFileID,
				w.AddedAt, w.LastTouchedAt, w.TouchedCount, w.SuccessCount, w.FailCount, int(w.SuccessPct), true)
			if err != nil {
				log.Println("migrate word", err.Error())
			}
		}
	}

	log.Println("GetBot")
	tgBot, err := tg.GetBot(cfg.TgToken)
	if err != nil {
		return fmt.Errorf("tg.GetBot: %w", err)
	}

	msgChan := make(chan tg.UserMsg)
	btnClickChan := make(chan tg.BtnClick)
	critErrosChan := make(chan error)

	// go func() {
	// 	if err := tgBot.GetUpdates(ctx, msgChan, btnClickChan); err != nil {
	// 		critErrosChan <- fmt.Errorf("tgBot.GetUpdates: %w", err)
	// 	}
	// }()

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
