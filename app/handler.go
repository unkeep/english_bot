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
	text = strings.ToLower(msg.Text)

	if msg.ChatID != h.cfg.TgChatID && msg.ChatID != h.cfg.TgAdminChatID {
		return fmt.Errorf("message from unknown chat: %+v", msg)
	}

	status, err := h.repo.Status.Get(ctx, fmt.Sprint(msg.ChatID))
	if err != nil {
		return fmt.Errorf("repo.Status.Get: %w", err)
	}

	// start/stop practicing
	if msg.Text == "/start_practicing" {
		w, err := h.repo.Words.PickOneToPractise(ctx)
		if err != nil {
			return fmt.Errorf("repo.Words.PickOneToPractise: %w", err)
		}
		status = db.Status{
			ID:     status.ID,
			Mode:   db.ModePractising,
			WordID: w.ID.Hex(),
		}
		if err := h.repo.Status.Save(ctx, status); err != nil {
			return fmt.Errorf("repo.Status.Save: %w", err)
		}

		if w.HintFileID != "" {
			_, err = h.tgBot.SendPhoto(msg.ChatID, w.HintFileID)
		} else {
			_, err = h.tgBot.SendMessage(tg.BotMessage{
				ChatID: msg.ChatID,
				Text:   w.Hint,
			})
		}
		if err != nil {
			return fmt.Errorf("tgBot.SendMessage/SendPhoto: %w", err)
		}

		return nil
	}

	if msg.Text == "/finish_practicing" {
		status = db.Status{
			ID:   status.ID,
			Mode: db.ModeNewWord,
		}
		if err := h.repo.Status.Save(ctx, status); err != nil {
			return fmt.Errorf("repo.Status.Save: %w", err)
		}
		return nil
	}

	// new word
	if status.Mode == db.ModeNewWord {
		id, err := h.repo.Words.AddNew(ctx, text)
		if err != nil {
			return fmt.Errorf("repo.Words.AddNew: %w", err)
		}

		status = db.Status{
			ID:     status.ID,
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
	if status.Mode == db.ModeHint {
		word, err := h.repo.Words.GetByID(ctx, status.WordID)
		if err != nil {
			return fmt.Errorf("repo.Words.GetByID: %w", err)
		}

		word.Hint = text
		word.HintFileID = msg.PhotoID
		err = h.repo.Words.Save(ctx, word)
		if err != nil {
			return fmt.Errorf("repo.Words.Save: %w", err)
		}

		err = h.tgBot.EditBtns(msg.ChatID, status.BtnMessageID, []tg.Btn{{
			ID:   fmt.Sprint(word.ID),
			Text: "✅ Edit hint",
		}})
		if err != nil {
			return fmt.Errorf("tgBot.EditBtns: %w", err)
		}

		status = db.Status{
			ID:   status.ID,
			Mode: db.ModeNewWord,
		}
		if err := h.repo.Status.Save(ctx, status); err != nil {
			return fmt.Errorf("repo.Status.Save: %w", err)
		}

		return nil
	}

	// practising
	if status.Mode == db.ModePractising {
		w, err := h.repo.Words.GetByID(ctx, status.WordID)
		if err != nil {
			return fmt.Errorf("h.repo.Words.GetByID: %w", err)
		}
		w.LastTouchedAt = time.Now().Unix()
		w.TouchedCount++
		var reply string
		var needNewWord bool
		if w.Text == text {
			w.SuccessCount++
			reply = "✅"
			needNewWord = true
		} else if text == "/giveup" {
			reply = "☹️ The correct answer is: " + w.Text + "\n"
			w.FailCount++
			needNewWord = true
		} else {
			w.FailCount++
			reply = "❌"
		}

		w.SuccessPct = float32(w.SuccessCount) / float32(w.TouchedCount) * 100.0

		if err := h.repo.Words.Save(ctx, w); err != nil {
			return fmt.Errorf("repo.Words.Save: %w", err)
		}

		var newWordHintFileID string
		if needNewWord {
			newW, err := h.repo.Words.PickOneToPractise(ctx)
			if err != nil {
				return fmt.Errorf("repo.Words.PickOneToPractise: %w", err)
			}
			if newW.Hint != "" {
				reply += "\n" + newW.Hint
			}
			newWordHintFileID = newW.HintFileID

			status.WordID = newW.ID.Hex()
			if h.repo.Status.Save(ctx, status); err != nil {
				return fmt.Errorf("repo.Status.Save: %w", err)
			}
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

		if newWordHintFileID != "" {
			time.Sleep(time.Millisecond * 500)
			_, err = h.tgBot.SendPhoto(msg.ChatID, newWordHintFileID)
			if err != nil {
				return fmt.Errorf("tgBot.SendPhoto: %w", err)
			}
		}

		return nil
	}

	return nil
}

func (h *handler) handleBtnClickMessage(ctx context.Context, click tg.BtnClick) error {
	status, err := h.repo.Status.Get(ctx, fmt.Sprint(click.Msg.ChatID))
	if err != nil {
		return fmt.Errorf("h.repo.Status.Get: %w", err)
	}

	word, err := h.repo.Words.GetByID(ctx, click.BtnID)
	if err != nil {
		return fmt.Errorf("repo.Words.GetByID: %w", err)
	}

	status.Mode = db.ModeHint
	status.WordID = word.ID.Hex()
	status.BtnMessageID = click.Msg.ID

	if err := h.repo.Status.Save(ctx, status); err != nil {
		return fmt.Errorf("h.repo.Status.Save: %w", err)
	}

	_, err = h.tgBot.SendMessage(tg.BotMessage{
		ChatID:       click.Msg.ChatID,
		Text:         fmt.Sprintf("Give a hint for %q", word.Text),
		TextMarkdown: false,
	})
	if err != nil {
		return fmt.Errorf("tgBot.SendMessage: %w", err)
	}

	return nil
}
