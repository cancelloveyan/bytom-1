package api

import (
	"context"

	chainjson "github.com/bytom/encoding/json"
	"github.com/bytom/errors"
	"github.com/bytom/mining"
	"github.com/bytom/protocol/bc"
	"github.com/bytom/protocol/bc/types"
)

// TODO
type GbtReq struct {
	Capabilities []string `json:"capabilities,omitempty"`
	Mode         string   `json:"mode,omitempty"`

	// Optional long polling.
	LongPollID string `json:"longpollid,omitempty"`

	// Optional template tweaking.  SigOpLimit and SizeLimit can be int64
	// or bool.
	SigOpLimit interface{} `json:"sigoplimit,omitempty"`
	SizeLimit  interface{} `json:"sizelimit,omitempty"`
	MaxVersion uint32      `json:"maxversion,omitempty"`

	// Basic pool extension from BIP 0023.
	Target string `json:"target,omitempty"`

	// Block proposal from BIP 0023.  Data is only provided when Mode is
	// "proposal".
	Data   string `json:"data,omitempty"`
	WorkID string `json:"workid,omitempty"`
}

func (a *API) getBlockTemplate(ins *GbtReq) Response {
	mode := "template" // Default mode: template
	if ins.Mode != "" {
		mode = ins.Mode
	}

	switch mode {
	case "template":
		return a.handleGbtRequest(ins)
	case "proposal":
		return a.handleGbtProposal(ins)
	}
	return NewErrorResponse(errors.New("Invalid mode."))
}

// BlockHeaderJSON struct provides support for get work in json format, when it also follows
// BlockHeader structure
type BlockHeaderJSON struct {
	Version           uint64                 `json:"version"`             // The version of the block.
	Height            uint64                 `json:"height"`              // The height of the block.
	PreviousBlockHash bc.Hash                `json:"previous_block_hash"` // The hash of the previous block.
	Timestamp         uint64                 `json:"timestamp"`           // The time of the block in seconds.
	Nonce             uint64                 `json:"nonce"`               // Nonce used to generate the block.
	Bits              uint64                 `json:"bits"`                // Difficulty target for the block.
	BlockCommitment   *types.BlockCommitment `json:"block_commitment"`    // Block commitment
}

type CoinbaseArbitrary struct {
	Arbitrary chainjson.HexBytes `json:"arbitrary"`
}

func (a *API) getCoinbaseArbitrary() Response {
	arbitrary := a.wallet.AccountMgr.GetCoinbaseArbitrary()
	resp := &CoinbaseArbitrary{
		Arbitrary: arbitrary,
	}
	return NewSuccessResponse(resp)
}

func (a *API) setCoinbaseArbitrary(ctx context.Context, req CoinbaseArbitrary) Response {
	a.wallet.AccountMgr.SetCoinbaseArbitrary(req.Arbitrary)
	return a.getCoinbaseArbitrary()
}

// getWork gets work in compressed protobuf format
func (a *API) getWork() Response {
	work, err := a.GetWork()
	if err != nil {
		return NewErrorResponse(err)
	}
	return NewSuccessResponse(work)
}

// getWorkJSON gets work in json format
func (a *API) getWorkJSON() Response {
	work, err := a.GetWorkJSON()
	if err != nil {
		return NewErrorResponse(err)
	}
	return NewSuccessResponse(work)
}

// SubmitWorkJSONReq is req struct for submit-work API
type SubmitWorkReq struct {
	BlockHeader *types.BlockHeader `json:"block_header"`
}

// submitWork submits work in compressed protobuf format
func (a *API) submitWork(ctx context.Context, req *SubmitWorkReq) Response {
	if err := a.SubmitWork(req.BlockHeader); err != nil {
		return NewErrorResponse(err)
	}
	return NewSuccessResponse(true)
}

// SubmitWorkJSONReq is req struct for submit-work-json API
type SubmitWorkJSONReq struct {
	BlockHeader *BlockHeaderJSON `json:"block_header"`
}

// submitWorkJSON submits work in json format
func (a *API) submitWorkJSON(ctx context.Context, req *SubmitWorkJSONReq) Response {
	bh := &types.BlockHeader{
		Version:           req.BlockHeader.Version,
		Height:            req.BlockHeader.Height,
		PreviousBlockHash: req.BlockHeader.PreviousBlockHash,
		Timestamp:         req.BlockHeader.Timestamp,
		Nonce:             req.BlockHeader.Nonce,
		Bits:              req.BlockHeader.Bits,
		BlockCommitment:   *req.BlockHeader.BlockCommitment,
	}

	if err := a.SubmitWork(bh); err != nil {
		return NewErrorResponse(err)
	}
	return NewSuccessResponse(true)
}

// GetWorkResp is resp struct for get-work API
type GetWorkResp struct {
	BlockHeader *types.BlockHeader `json:"block_header"`
	Seed        *bc.Hash           `json:"seed"`
}

// GetWork gets work in compressed protobuf format
func (a *API) GetWork() (*GetWorkResp, error) {
	bh, err := a.miningPool.GetWork()
	if err != nil {
		return nil, err
	}

	seed, err := a.chain.CalcNextSeed(&bh.PreviousBlockHash)
	if err != nil {
		return nil, err
	}

	return &GetWorkResp{
		BlockHeader: bh,
		Seed:        seed,
	}, nil
}

// GetWorkJSONResp is resp struct for get-work-json API
type GetWorkJSONResp struct {
	BlockHeader *BlockHeaderJSON `json:"block_header"`
	Seed        *bc.Hash         `json:"seed"`
}

// GetWorkJSON gets work in json format
func (a *API) GetWorkJSON() (*GetWorkJSONResp, error) {
	bh, err := a.miningPool.GetWork()
	if err != nil {
		return nil, err
	}

	seed, err := a.chain.CalcNextSeed(&bh.PreviousBlockHash)
	if err != nil {
		return nil, err
	}

	return &GetWorkJSONResp{
		BlockHeader: &BlockHeaderJSON{
			Version:           bh.Version,
			Height:            bh.Height,
			PreviousBlockHash: bh.PreviousBlockHash,
			Timestamp:         bh.Timestamp,
			Nonce:             bh.Nonce,
			Bits:              bh.Bits,
			BlockCommitment:   &bh.BlockCommitment,
		},
		Seed: seed,
	}, nil
}

// SubmitWork tries to submit work to the chain
func (a *API) SubmitWork(bh *types.BlockHeader) error {
	return a.miningPool.SubmitWork(bh)
}

func (a *API) setMining(in struct {
	IsMining bool `json:"is_mining"`
}) Response {
	if in.IsMining {
		if _, err := a.wallet.AccountMgr.GetMiningAddress(); err != nil {
			return NewErrorResponse(errors.New("Mining address does not exist"))
		}
		return a.startMining()
	}
	return a.stopMining()
}

func (a *API) startMining() Response {
	a.cpuMiner.Start()
	if !a.IsMining() {
		return NewErrorResponse(errors.New("Failed to start mining"))
	}
	return NewSuccessResponse("")
}

func (a *API) stopMining() Response {
	a.cpuMiner.Stop()
	if a.IsMining() {
		return NewErrorResponse(errors.New("Failed to stop mining"))
	}
	return NewSuccessResponse("")
}

// TODO
func (a *API) submitBlock(b mining.BlockTemplate) Response {
	a.miningPool.SubmitBlock(b)
	return NewSuccessResponse("")
	return NewErrorResponse(errors.New("submit-block not implemented yet."))
}

func (a *API) handleGbtRequest(ins *GbtReq) Response {
	// Extract the relevant passed capabilities and restrict the result to
	// either a coinbase value or a coinbase transaction object depending on
	// the request.  Default to only providing a coinbase value.
	useCoinbaseValue := true
	var hasCoinbaseValue, hasCoinbaseTxn bool
	for _, capability := range ins.Capabilities {
		switch capability {
		case "coinbasetxn":
			hasCoinbaseTxn = true
		case "coinbasevalue":
			hasCoinbaseValue = true
		}
	}
	if hasCoinbaseTxn && !hasCoinbaseValue {
		useCoinbaseValue = false
	}
	// TODO
	if useCoinbaseValue {
		// return NewErrorResponse(errors.New("state.blockTemplateResult(useCoinbaseValue, nil) not implemented yet."))
	}

	// TODO
	// When a long poll ID was provided, this is a long poll request by the
	// client to be notified when block template referenced by the ID should
	// be replaced with a new one.
	if ins.LongPollID != "" {
		return NewErrorResponse(errors.New("long poll not supported yet."))
	}

	// TODO
	// Protect concurrent access when updating block templates.
	/*
		state := s.gbtWorkState
		state.Lock()
		defer state.Unlock()
	*/

	// TODO
	// Get and return a block template.  A new block template will be
	// generated when the current best block has changed or the transactions
	// in the memory pool have been updated and it has been at least five
	// seconds since the last template was generated.  Otherwise, the
	// timestamp for the existing block template is updated (and possibly
	// the difficulty on testnet per the consesus rules).
	/*
		if err := state.updateBlockTemplate(s, useCoinbaseValue); err != nil {
			return nil, err
		}
	*/

	template := a.miningPool.GetBlockTemplate()
	if template == nil {
		return NewErrorResponse(errors.New("block template not ready yet."))
	} else {
		return NewSuccessResponse(template)
	}
}

func (a *API) handleGbtProposal(ins *GbtReq) Response {
	// TODO
	return NewErrorResponse(errors.New("handleGbtProposal() not implemented yet."))
}
