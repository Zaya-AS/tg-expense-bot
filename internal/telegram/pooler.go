package telegram

import (
	"context"
	"fmt"
)

type Poller struct {
	client  *Client
	handler *Handler
}

func NewPoller(client *Client, handler *Handler) *Poller {
	return &Poller{
		client:  client,
		handler: handler,
	}
}

func (p *Poller) Run(ctx context.Context) error {

	var offset int64

	for {
		if ctx.Err() != nil {
			return nil
		}

		updates, err := p.client.GetUpdates(ctx, offset)

		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("get Telegram updates: %w", err)
		}

		for _, update := range updates {
			if err := p.handler.Handle(ctx, update); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return fmt.Errorf("handle update %d: %w", update.UpdateID, err)
			}
			offset = update.UpdateID + 1
		}
	}

}
