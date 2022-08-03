package app

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/unkeep/english_bot/db"
	"github.com/unkeep/english_bot/tg"
)

type handler struct {
	repo  *db.Repo
	tgBot *tg.Bot
	cfg   config
}

func (h *handler) handleUserMessage(ctx context.Context, msg tg.UserMsg) error {
	log.Println(msg)

	text := strings.TrimSpace(msg.Text)

	if msg.ChatID != h.cfg.TgChatID && msg.ChatID != h.cfg.TgAdminChatID {
		return fmt.Errorf("message from unknown chat: %+v", msg)
	}

	status, err := h.repo.Status.Get(ctx)
	if err != nil {
		return fmt.Errorf("repo.Status.Get: %w", err)
	}

	// start/stop practicing
	if msg.ChatID == h.cfg.TgAdminChatID {
		if msg.Text == "/start_practicing" {
			w, err := h.repo.Words.PickOneToPractise(ctx)
			if err != nil {
				return fmt.Errorf("repo.Words.PickOneToPractise: %w", err)
			}
			status = db.Status{
				Mode:   db.ModePractising,
				WordID: w.ID.Hex(),
			}
			if err := h.repo.Status.Save(ctx, status); err != nil {
				return fmt.Errorf("repo.Status.Save: %w", err)
			}

			_, err = h.tgBot.SendMessage(tg.BotMessage{
				ChatID:       msg.ChatID,
				ReplyToMsgID: 0,
				Text:         fmt.Sprintf("Hint is:\n%s", w.Hint),
				TextMarkdown: false,
			})
			if err != nil {
				return fmt.Errorf("tgBot.SendMessage: %w", err)
			}

			return nil
		}

		if msg.Text == "/finish_practicing" {
			status = db.Status{
				Mode: db.ModeNewWord,
			}
			if err := h.repo.Status.Save(ctx, status); err != nil {
				return fmt.Errorf("repo.Status.Save: %w", err)
			}
			return nil
		}
	}

	// new word
	if msg.ChatID == h.cfg.TgChatID && status.Mode == db.ModeNewWord {
		id, err := h.repo.Words.AddNew(ctx, text)
		if err != nil {
			return fmt.Errorf("repo.Words.AddNew: %w", err)
		}

		status = db.Status{
			Mode:   db.ModeHint,
			WordID: id,
		}
		if err := h.repo.Status.Save(ctx, status); err != nil {
			return fmt.Errorf("repo.Status.Save: %w", err)
		}

		_, err = h.tgBot.SendMessage(tg.BotMessage{
			ChatID:       msg.ChatID,
			ReplyToMsgID: msg.ID,
			Text:         "added!",
			TextMarkdown: false,
			Btns: []tg.Btn{{
				ID:   id,
				Text: "Set hint",
			}},
		})
		if err != nil {
			return fmt.Errorf("tgBot.SendMessage: %w", err)
		}
		return nil
	}

	// set hint
	if msg.ChatID == h.cfg.TgChatID && status.Mode == db.ModeHint {
		word, err := h.repo.Words.GetByID(ctx, status.WordID)
		if err != nil {
			return fmt.Errorf("repo.Words.GetByID: %w", err)
		}

		word.Hint = text
		err = h.repo.Words.Save(ctx, word)
		if err != nil {
			return fmt.Errorf("repo.Words.Save: %w", err)
		}

		status = db.Status{
			Mode: db.ModeNewWord,
		}
		if err := h.repo.Status.Save(ctx, status); err != nil {
			return fmt.Errorf("repo.Status.Save: %w", err)
		}

		return nil
	}

	// practising
	if msg.ChatID == h.cfg.TgAdminChatID && status.Mode == db.ModePractising {
		w, err := h.repo.Words.GetByID(ctx, status.WordID)
		if err != nil {
			return fmt.Errorf("h.repo.Words.GetByID: %w", err)
		}
		w.LastTouchedAt = time.Now().Unix()
		w.TouchedCount++
		var reply string
		if w.Text == text {
			w.SuccessCount++
			reply = "Correct!\n"

			newW, err := h.repo.Words.PickOneToPractise(ctx)
			if err != nil {
				return fmt.Errorf("repo.Words.PickOneToPractise: %w", err)
			}
			reply += "Hint is: " + newW.Hint
			status.WordID = newW.ID.Hex()
		} else {
			w.FailCount++
			reply = "Wrong!"
		}

		if err := h.repo.Words.Save(ctx, w); err != nil {
			return fmt.Errorf("repo.Words.Save: %w", err)
		}

		if h.repo.Status.Save(ctx, status); err != nil {
			return fmt.Errorf("repo.Status.Save: %w", err)
		}

		_, err = h.tgBot.SendMessage(tg.BotMessage{
			ChatID:       msg.ChatID,
			ReplyToMsgID: 0,
			Text:         reply,
			TextMarkdown: false,
		})
		if err != nil {
			return fmt.Errorf("tgBot.SendMessage: %w", err)
		}

		return nil
	}

	return nil
}

func (h *handler) handleBtnClickMessage(ctx context.Context, click tg.BtnClick) error {
	status, err := h.repo.Status.Get(ctx)
	if err != nil {
		return fmt.Errorf("h.repo.Status.Get: %w", err)
	}

	word, err := h.repo.Words.GetByID(ctx, click.BtnID)
	if err != nil {
		return fmt.Errorf("repo.Words.GetByID: %w", err)
	}

	status.Mode = db.ModeHint
	status.WordID = word.ID.Hex()

	if err := h.repo.Status.Save(ctx, status); err != nil {
		return fmt.Errorf("h.repo.Status.Save: %w", err)
	}

	_, err = h.tgBot.SendMessage(tg.BotMessage{
		ChatID:       h.cfg.TgChatID,
		Text:         fmt.Sprintf("write a hint for %q", word.Text),
		TextMarkdown: false,
	})
	if err != nil {
		return fmt.Errorf("tgBot.SendMessage: %w", err)
	}

	return nil
}
