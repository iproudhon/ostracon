package state_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/line/tm-db/v2/memdb"
	"github.com/line/tm-db/v2/metadb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/line/ostracon/abci/types"
	cfg "github.com/line/ostracon/config"
	"github.com/line/ostracon/crypto"
	"github.com/line/ostracon/crypto/ed25519"
	tmrand "github.com/line/ostracon/libs/rand"
	tmstate "github.com/line/ostracon/proto/ostracon/state"
	tmproto "github.com/line/ostracon/proto/ostracon/types"
	sm "github.com/line/ostracon/state"
	"github.com/line/ostracon/types"
)

func TestStoreLoadValidators(t *testing.T) {
	stateDB := memdb.NewDB()
	stateStore := sm.NewStore(stateDB)
	val, _ := types.RandValidator(true, 10)
	vals := types.NewValidatorSet([]*types.Validator{val})

	// 1) LoadValidators loads validators using a height where they were last changed
	err := sm.SaveValidatorsInfo(stateDB, 1, 1, []byte{}, vals)
	require.NoError(t, err)
	err = sm.SaveValidatorsInfo(stateDB, 2, 1, []byte{}, vals)
	require.NoError(t, err)
	loadedVals, err := stateStore.LoadValidators(2)
	require.NoError(t, err)
	assert.NotZero(t, loadedVals.Size())

	// 2) LoadValidators loads validators using a checkpoint height

	err = sm.SaveValidatorsInfo(stateDB, sm.ValSetCheckpointInterval, 1, []byte{}, vals)
	require.NoError(t, err)

	loadedVals, err = stateStore.LoadValidators(sm.ValSetCheckpointInterval)
	require.NoError(t, err)
	assert.NotZero(t, loadedVals.Size())
}

func TestStoreLoadVoters(t *testing.T) {
	stateDB := memdb.NewDB()
	stateStore := sm.NewStore(stateDB)
	val, _ := types.RandValidator(true, 10)
	vals := types.NewValidatorSet([]*types.Validator{val})

	height := int64(1)

	state := createState(height, 1, 1, vals)
	err := stateStore.Save(state)
	require.NoError(t, err)

	{
		_, loadedVoters, _, _, err := stateStore.LoadVoters(height, nil)
		require.NoError(t, err)
		assert.Equal(t, vals.Size(), loadedVoters.Size())
	}
	{
		_, _, _, _, err := stateStore.LoadVoters(height-1, nil)
		require.Error(t, err)
		require.Equal(t, sm.ErrNoValSetForHeight{Height: height - 1}, err)
	}
	{
		_, _, _, _, err := stateStore.LoadVoters(height+1, nil)
		require.Error(t, err)
		require.Error(t, sm.ErrNoValSetForHeight{Height: height + 1}, err)
	}

	state.Validators = types.NewValidatorSet([]*types.Validator{})
	err = stateStore.Save(state)
	require.NoError(t, err)

	{
		_, _, _, _, err := stateStore.LoadVoters(height, nil)
		require.Error(t, err)
		require.Equal(t, "validator set is nil or empty", err.Error())
	}
	{
		_, _, _, _, err := stateStore.LoadVoters(height+1, nil)
		require.Error(t, err)
		require.Equal(t, "validator set is nil or empty", err.Error())
	}

	err = sm.SaveValidatorsInfo(stateDB, height, height, []byte{}, types.NewValidatorSet([]*types.Validator{val}))
	require.NoError(t, err)

	height++
	{
		_, _, _, _, err := stateStore.LoadVoters(height, nil)
		require.Error(t, err)
		require.Equal(t, sm.ErrNoVoterParamsForHeight{Height: height}, err)
	}

	err = sm.SaveVoterParams(stateDB, height, types.DefaultVoterParams())
	require.NoError(t, err)

	{
		_, _, _, _, err := stateStore.LoadVoters(height, nil)
		require.Error(t, err)
		require.Equal(t, sm.ErrNoProofHashForHeight{Height: height}, err)
	}

	err = sm.SaveProofHash(stateDB, height, []byte{0})
	require.NoError(t, err)

	{
		_, loadedVoters, _, _, err := stateStore.LoadVoters(height, nil)
		require.NoError(t, err)
		assert.Equal(t, vals.Size(), loadedVoters.Size())
	}
}

func BenchmarkLoadValidators(b *testing.B) {
	const valSetSize = 100

	config := cfg.ResetTestRoot("state_")
	defer os.RemoveAll(config.RootDir)
	dbType := metadb.BackendType(config.DBBackend)
	stateDB, err := metadb.NewDB("state", dbType, config.DBDir())
	require.NoError(b, err)
	stateStore := sm.NewStore(stateDB)
	state, err := stateStore.LoadFromDBOrGenesisFile(config.GenesisFile())
	if err != nil {
		b.Fatal(err)
	}

	state.Validators = genValSet(valSetSize)
	state.Validators.SelectProposer([]byte{}, 1, 0)
	state.NextValidators = state.Validators.Copy()
	state.NextValidators.SelectProposer([]byte{}, 2, 0)
	err = stateStore.Save(state)
	require.NoError(b, err)

	for i := 10; i < 10000000000; i *= 10 { // 10, 100, 1000, ...
		i := i
		if err := sm.SaveValidatorsInfo(stateDB,
			int64(i), state.LastHeightValidatorsChanged, []byte{}, state.NextValidators); err != nil {
			b.Fatal(err)
		}

		b.Run(fmt.Sprintf("height=%d", i), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				_, err := stateStore.LoadValidators(int64(i))
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func createState(height, valsChanged, paramsChanged int64, validatorSet *types.ValidatorSet) sm.State {
	if height < 1 {
		panic(height)
	}
	state := sm.State{
		InitialHeight:   1,
		VoterParams:     types.DefaultVoterParams(),
		LastBlockHeight: height - 1,
		Validators:      validatorSet,
		NextValidators:  validatorSet,
		Voters:          types.ToVoterAll(validatorSet.Validators),
		ConsensusParams: tmproto.ConsensusParams{
			Block: tmproto.BlockParams{MaxBytes: 10e6},
		},
		LastHeightValidatorsChanged:      valsChanged,
		LastHeightConsensusParamsChanged: paramsChanged,
		LastProofHash:                    []byte{0},
	}
	if state.LastBlockHeight >= 1 {
		state.LastVoters = state.Voters
	}
	return state
}

func TestPruneStates(t *testing.T) {
	testcases := map[string]struct {
		makeHeights  int64
		pruneFrom    int64
		pruneTo      int64
		expectErr    bool
		expectVals   []int64
		expectParams []int64
		expectABCI   []int64
	}{
		"error on pruning from 0":      {100, 0, 5, true, nil, nil, nil},
		"error when from > to":         {100, 3, 2, true, nil, nil, nil},
		"error when from == to":        {100, 3, 3, true, nil, nil, nil},
		"error when to does not exist": {100, 1, 101, true, nil, nil, nil},
		"prune all":                    {100, 1, 100, false, []int64{93, 100}, []int64{95, 100}, []int64{100}},
		"prune some": {10, 2, 8, false, []int64{1, 3, 8, 9, 10},
			[]int64{1, 5, 8, 9, 10}, []int64{1, 8, 9, 10}},
		"prune across checkpoint": {100001, 1, 100001, false, []int64{99993, 100000, 100001},
			[]int64{99995, 100001}, []int64{100001}},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			db := memdb.NewDB()
			stateStore := sm.NewStore(db)
			pk := ed25519.GenPrivKey().PubKey()

			// Generate a bunch of state data. Validators change for heights ending with 3, and
			// parameters when ending with 5.
			validator := &types.Validator{Address: tmrand.Bytes(crypto.AddressSize), StakingPower: 100, PubKey: pk}
			validatorSet := &types.ValidatorSet{
				Validators: []*types.Validator{validator},
			}
			valsChanged := int64(0)
			paramsChanged := int64(0)

			for h := int64(1); h <= tc.makeHeights; h++ {
				if valsChanged == 0 || h%10 == 2 {
					valsChanged = h + 1 // Have to add 1, since NextValidators is what's stored
				}
				if paramsChanged == 0 || h%10 == 5 {
					paramsChanged = h
				}

				state := createState(h, valsChanged, paramsChanged, validatorSet)

				err := stateStore.Save(state)
				require.NoError(t, err)

				err = stateStore.SaveABCIResponses(h, &tmstate.ABCIResponses{
					DeliverTxs: []*abci.ResponseDeliverTx{
						{Data: []byte{1}},
						{Data: []byte{2}},
						{Data: []byte{3}},
					},
				})
				require.NoError(t, err)
			}

			// Test assertions
			err := stateStore.PruneStates(tc.pruneFrom, tc.pruneTo)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			expectVals := sliceToMap(tc.expectVals)
			expectParams := sliceToMap(tc.expectParams)
			expectABCI := sliceToMap(tc.expectABCI)

			for h := int64(1); h <= tc.makeHeights; h++ {
				vals, err := stateStore.LoadValidators(h)
				if expectVals[h] {
					require.NoError(t, err, "validators height %v", h)
					require.NotNil(t, vals)
				} else {
					require.Error(t, err, "validators height %v", h)
					require.Equal(t, sm.ErrNoValSetForHeight{Height: h}, err)
				}

				params, err := stateStore.LoadConsensusParams(h)
				if expectParams[h] {
					require.NoError(t, err, "params height %v", h)
					require.False(t, params.Equal(&tmproto.ConsensusParams{}))
				} else {
					require.Error(t, err, "params height %v", h)
				}

				abci, err := stateStore.LoadABCIResponses(h)
				if expectABCI[h] {
					require.NoError(t, err, "abci height %v", h)
					require.NotNil(t, abci)
				} else {
					require.Error(t, err, "abci height %v", h)
					require.Equal(t, sm.ErrNoABCIResponsesForHeight{Height: h}, err)
				}
			}
		})
	}
}

func TestABCIResponsesResultsHash(t *testing.T) {
	responses := &tmstate.ABCIResponses{
		BeginBlock: &abci.ResponseBeginBlock{},
		DeliverTxs: []*abci.ResponseDeliverTx{
			{Code: 32, Data: []byte("Hello"), Log: "Huh?"},
		},
		EndBlock: &abci.ResponseEndBlock{},
	}

	root := sm.ABCIResponsesResultsHash(responses)

	// root should be Merkle tree root of DeliverTxs responses
	results := types.NewResults(responses.DeliverTxs)
	assert.Equal(t, root, results.Hash())

	// test we can prove first DeliverTx
	proof := results.ProveResult(0)
	bz, err := results[0].Marshal()
	require.NoError(t, err)
	assert.NoError(t, proof.Verify(root, bz))
}

func sliceToMap(s []int64) map[int64]bool {
	m := make(map[int64]bool, len(s))
	for _, i := range s {
		m[i] = true
	}
	return m
}
