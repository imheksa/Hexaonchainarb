package scanner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

const telegramAPIBase = "https://api.telegram.org/bot%s/sendMessage"

// TelegramConfig holds Telegram bot credentials.
type TelegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
	// MinProfitBPS overrides the global threshold for notifications only.
	// 0 means use the global scanner threshold.
	MinProfitBPS int64 `json:"min_profit_bps"`
}

// TelegramNotifier sends arbitrage opportunities to a Telegram chat.
type TelegramNotifier struct {
	cfg    TelegramConfig
	client *http.Client
	queue  chan *Opportunity
}

func NewTelegramNotifier(cfg TelegramConfig) *TelegramNotifier {
	return &TelegramNotifier{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
		queue:  make(chan *Opportunity, 64),
	}
}

// Notify enqueues an opportunity for sending (non-blocking).
func (t *TelegramNotifier) Notify(opp *Opportunity) {
	if t.cfg.MinProfitBPS > 0 && opp.ProfitBPS < t.cfg.MinProfitBPS {
		return
	}
	select {
	case t.queue <- opp:
	default:
		log.Warn().Msg("Telegram queue full, dropping notification")
	}
}

// Run processes the notification queue until ctx is cancelled.
func (t *TelegramNotifier) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case opp := <-t.queue:
			if err := t.send(opp); err != nil {
				log.Warn().Err(err).Msg("Failed to send Telegram notification")
			}
		}
	}
}

func (t *TelegramNotifier) send(opp *Opportunity) error {
	text := formatMessage(opp)

	body, err := json.Marshal(map[string]any{
		"chat_id":    t.cfg.ChatID,
		"text":       text,
		"parse_mode": "HTML",
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf(telegramAPIBase, t.cfg.BotToken)
	resp, err := t.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned %d", resp.StatusCode)
	}
	return nil
}

func formatMessage(opp *Opportunity) string {
	kind := "🔄 Intra-Chain"
	warning := ""
	if opp.CrossChain {
		kind = "🌉 Cross-Chain"
		warning = "\n⚠️ <i>Bridge fee belum termasuk — cek manual sebelum eksekusi</i>"
	}

	profitPct := float64(opp.ProfitBPS) / 100.0
	spreadPct := float64(opp.SpreadBPS) / 100.0

	return fmt.Sprintf(
		`🚨 <b>Arbitrage Opportunity</b> %s

<b>Pair</b>   : <code>%s</code>
<b>Buy</b>    : %s @ %s
<b>Price</b>  : <code>%.6f</code> (fee %d bps)

<b>Sell</b>   : %s @ %s
<b>Price</b>  : <code>%.6f</code> (fee %d bps)

<b>Spread</b> : %.2f%%  |  <b>Profit</b>: <b>%.2f%%</b> (%d bps)

<b>Buy Pool</b> : <code>%s</code>
<b>Sell Pool</b>: <code>%s</code>

🕐 %s%s`,
		kind,
		opp.Symbol,
		opp.BuyDEX, opp.BuyChain,
		opp.BuyPrice, opp.BuyFeeBPS,
		opp.SellDEX, opp.SellChain,
		opp.SellPrice, opp.SellFeeBPS,
		spreadPct, profitPct, opp.ProfitBPS,
		opp.BuyPool,
		opp.SellPool,
		opp.DetectedAt.Format("2006-01-02 15:04:05 UTC"),
		warning,
	)
}
