package reassembly

import (
	"sync"
	"time"

	"httpinspector/models"
)

type ConnectionState struct {
	mu        sync.Mutex
	Tx        models.HTTPTransaction
	ReqParsed bool
	ResParsed bool
	ResTime   time.Time
}

type TransactionTracker struct {
	pending sync.Map
}

func NewTransactionTracker() *TransactionTracker {
	return &TransactionTracker{}
}

func (t *TransactionTracker) AddRequest(connID string, reqTx models.HTTPTransaction) (*models.HTTPTransaction, bool) {
	val, _ := t.pending.LoadOrStore(connID, &ConnectionState{})
	state := val.(*ConnectionState)

	state.mu.Lock()
	defer state.mu.Unlock()

	state.Tx.ID = reqTx.ID
	state.Tx.Method = reqTx.Method
	state.Tx.Host = reqTx.Host
	state.Tx.Path = reqTx.Path
	state.Tx.RequestTime = reqTx.RequestTime
	state.Tx.SourceIP = reqTx.SourceIP
	state.Tx.SourcePort = reqTx.SourcePort
	state.Tx.DestIP = reqTx.DestIP
	state.Tx.DestPort = reqTx.DestPort
	state.ReqParsed = true

	if state.ResParsed {
		t.pending.Delete(connID)
		state.Tx.Duration = state.ResTime.Sub(state.Tx.RequestTime)
		return &state.Tx, true
	}
	return nil, false
}

func (t *TransactionTracker) AddResponse(connID string, resTx models.HTTPTransaction, resTime time.Time) (*models.HTTPTransaction, bool) {
	val, _ := t.pending.LoadOrStore(connID, &ConnectionState{})
	state := val.(*ConnectionState)

	state.mu.Lock()
	defer state.mu.Unlock()

	state.Tx.StatusCode = resTx.StatusCode
	state.Tx.ContentLength = resTx.ContentLength
	state.ResTime = resTime
	state.ResParsed = true

	if state.ReqParsed {
		t.pending.Delete(connID)
		state.Tx.Duration = state.ResTime.Sub(state.Tx.RequestTime)
		return &state.Tx, true
	}
	return nil, false
}