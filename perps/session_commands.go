package perps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
)

type sessionResponse struct {
	data json.RawMessage
	err  error
}

type orderWaitResponse struct {
	update perpsOrderUpdate
	err    error
}

func (s *Session) resolveOrderWaiters(event PerpsSessionEvent) {
	if event.Channel != "orders" {
		return
	}
	var update perpsOrderUpdate
	if err := json.Unmarshal(event.Data, &update); err != nil {
		s.reportError(fmt.Errorf("perps: decode order update: %w", err))
		return
	}
	s.orderWaitMu.Lock()
	waiter := s.orderWaiters[update.ClientOrderID]
	delete(s.orderWaiters, update.ClientOrderID)
	s.orderWaitMu.Unlock()
	if waiter != nil {
		waiter <- orderWaitResponse{update: update}
	}
}

type orderWatch struct {
	clientID string
	response chan orderWaitResponse
}

func (s *Session) watchOrder(clientID string) (orderWatch, error) {
	s.orderWaitMu.Lock()
	defer s.orderWaitMu.Unlock()
	if s.orderWaiters == nil {
		s.orderWaiters = make(map[string]chan orderWaitResponse)
	}
	if _, exists := s.orderWaiters[clientID]; exists {
		return orderWatch{}, fmt.Errorf("perps: already waiting for client order ID")
	}
	watch := orderWatch{clientID: clientID, response: make(chan orderWaitResponse, 1)}
	s.orderWaiters[clientID] = watch.response
	return watch, nil
}

func (s *Session) unwatchOrder(watch orderWatch) {
	s.orderWaitMu.Lock()
	defer s.orderWaitMu.Unlock()
	if s.orderWaiters[watch.clientID] == watch.response {
		delete(s.orderWaiters, watch.clientID)
	}
}

func (s *Session) waitWatchedOrder(
	ctx context.Context,
	watch orderWatch,
	orderID int,
) (perpsOrderUpdate, error) {
	select {
	case result := <-watch.response:
		if result.err != nil {
			return perpsOrderUpdate{}, result.err
		}
		if result.update.ID != orderID {
			return perpsOrderUpdate{}, fmt.Errorf(
				"perps: acknowledgement and order update IDs differ",
			)
		}
		return result.update, nil
	case <-ctx.Done():
		return perpsOrderUpdate{}, ctx.Err()
	case <-s.ctx.Done():
		return perpsOrderUpdate{}, errors.New("perps session closed")
	}
}

func (s *Session) rejectOrderWaiters(err error) {
	s.orderWaitMu.Lock()
	waiters := slices.Collect(maps.Values(s.orderWaiters))
	s.orderWaiters = make(map[string]chan orderWaitResponse)
	s.orderWaitMu.Unlock()
	for _, waiter := range waiters {
		waiter <- orderWaitResponse{err: err}
	}
}

func (s *Session) resolvePending(id int, data json.RawMessage) bool {
	s.pendingMu.Lock()
	response, ok := s.pending[id]
	if ok {
		delete(s.pending, id)
	}
	s.pendingMu.Unlock()
	if !ok {
		return false
	}
	response <- sessionResponse{data: append(json.RawMessage(nil), data...)}
	return true
}

func (s *Session) rejectPending(err error) {
	s.pendingMu.Lock()
	responses := slices.Collect(maps.Values(s.pending))
	s.pending = make(map[int]chan sessionResponse)
	s.pendingMu.Unlock()
	for _, response := range responses {
		response <- sessionResponse{err: err}
	}
}

func (s *Session) nextID() int {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	id := s.nextRequest
	s.nextRequest++
	return id
}

func (s *Session) sendCommand(ctx context.Context, body map[string]any) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.ctx.Err(); err != nil {
		return nil, fmt.Errorf("perps session closed: %w", err)
	}
	id := s.nextID()
	body["id"] = id
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("perps: marshal session command: %w", err)
	}
	response := make(chan sessionResponse, 1)
	s.pendingMu.Lock()
	s.pending[id] = response
	s.pendingMu.Unlock()
	defer func() {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
	}()
	if err := s.writeRaw(ctx, payload); err != nil {
		return nil, fmt.Errorf("perps: send session command: %w", err)
	}
	select {
	case result := <-response:
		return result.data, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.ctx.Done():
		return nil, errors.New("perps session closed")
	}
}
