package keeper_test

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	upgradetypes "cosmossdk.io/x/upgrade/types"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/query"

	"github.com/cosmos/ibc-go/v8/modules/core/02-client/types"
	commitmenttypes "github.com/cosmos/ibc-go/v8/modules/core/23-commitment/types"
	host "github.com/cosmos/ibc-go/v8/modules/core/24-host"
	"github.com/cosmos/ibc-go/v8/modules/core/exported"
	ibctm "github.com/cosmos/ibc-go/v8/modules/light-clients/07-tendermint"
	ibctesting "github.com/cosmos/ibc-go/v8/testing"
	"github.com/cosmos/ibc-go/v8/testing/mock"
)

func (suite *KeeperTestSuite) TestQueryClientState() {
	var (
		req            *types.QueryClientStateRequest
		expClientState *codectypes.Any
	)

	testCases := []struct {
		msg      string
		malleate func()
		expPass  bool
	}{
		{
			"req is nil",
			func() {
				req = nil
			},
			false,
		},
		{
			"invalid clientID",
			func() {
				req = &types.QueryClientStateRequest{}
			},
			false,
		},
		{
			"client not found",
			func() {
				req = &types.QueryClientStateRequest{
					ClientId: testClientID,
				}
			},
			false,
		},
		{
			"success",
			func() {
				path := ibctesting.NewPath(suite.chainA, suite.chainB)
				path.SetupClients()

				var err error
				expClientState, err = types.PackClientState(path.EndpointA.GetClientState())
				suite.Require().NoError(err)

				req = &types.QueryClientStateRequest{
					ClientId: path.EndpointA.ClientID,
				}
			},
			true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			suite.SetupTest() // reset

			tc.malleate()
			ctx := suite.chainA.GetContext()
			res, err := suite.chainA.QueryServer.ClientState(ctx, req)

			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().NotNil(res)
				suite.Require().Equal(expClientState, res.ClientState)

				// ensure UnpackInterfaces is defined
				cachedValue := res.ClientState.GetCachedValue()
				suite.Require().NotNil(cachedValue)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestQueryClientStates() {
	var (
		req             *types.QueryClientStatesRequest
		expClientStates = types.IdentifiedClientStates{}
	)

	testCases := []struct {
		msg      string
		malleate func()
		expPass  bool
	}{
		{
			"req is nil",
			func() {
				req = nil
			},
			false,
		},
		{
			"empty pagination",
			func() {
				localhost := types.NewIdentifiedClientState(exported.LocalhostClientID, suite.chainA.GetClientState(exported.LocalhostClientID))
				expClientStates = types.IdentifiedClientStates{localhost}
				req = &types.QueryClientStatesRequest{}
			},
			true,
		},
		{
			"success, only localhost",
			func() {
				localhost := types.NewIdentifiedClientState(exported.LocalhostClientID, suite.chainA.GetClientState(exported.LocalhostClientID))
				expClientStates = types.IdentifiedClientStates{localhost}
				req = &types.QueryClientStatesRequest{
					Pagination: &query.PageRequest{
						Limit:      3,
						CountTotal: true,
					},
				}
			},
			true,
		},
		{
			"success",
			func() {
				path1 := ibctesting.NewPath(suite.chainA, suite.chainB)
				path1.SetupClients()

				path2 := ibctesting.NewPath(suite.chainA, suite.chainB)
				path2.SetupClients()

				clientStateA1 := path1.EndpointA.GetClientState()
				clientStateA2 := path2.EndpointA.GetClientState()

				localhost := types.NewIdentifiedClientState(exported.LocalhostClientID, suite.chainA.GetClientState(exported.LocalhostClientID))
				idcs := types.NewIdentifiedClientState(path1.EndpointA.ClientID, clientStateA1)
				idcs2 := types.NewIdentifiedClientState(path2.EndpointA.ClientID, clientStateA2)

				// order is sorted by client id
				expClientStates = types.IdentifiedClientStates{localhost, idcs, idcs2}.Sort()
				req = &types.QueryClientStatesRequest{
					Pagination: &query.PageRequest{
						Limit:      20,
						CountTotal: true,
					},
				}
			},
			true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			suite.SetupTest() // reset
			tc.malleate()

			ctx := suite.chainA.GetContext()
			res, err := suite.chainA.QueryServer.ClientStates(ctx, req)
			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().NotNil(res)
				suite.Require().Equal(expClientStates.Sort(), res.ClientStates)
				suite.Require().Equal(len(expClientStates), int(res.Pagination.Total))
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestQueryConsensusState() {
	var (
		req               *types.QueryConsensusStateRequest
		expConsensusState *codectypes.Any
	)

	testCases := []struct {
		msg      string
		malleate func()
		expPass  bool
	}{
		{
			"req is nil",
			func() {
				req = nil
			},
			false,
		},
		{
			"invalid clientID",
			func() {
				req = &types.QueryConsensusStateRequest{}
			},
			false,
		},
		{
			"invalid height",
			func() {
				req = &types.QueryConsensusStateRequest{
					ClientId:       testClientID,
					RevisionNumber: 0,
					RevisionHeight: 0,
					LatestHeight:   false,
				}
			},
			false,
		},
		{
			"consensus state not found",
			func() {
				req = &types.QueryConsensusStateRequest{
					ClientId:     ibctesting.FirstClientID,
					LatestHeight: true,
				}
			},
			false,
		},
		{
			"success latest height",
			func() {
				path := ibctesting.NewPath(suite.chainA, suite.chainB)
				path.SetupClients()
				cs := path.EndpointA.GetConsensusState(path.EndpointA.GetClientState().GetLatestHeight())

				var err error
				expConsensusState, err = types.PackConsensusState(cs)
				suite.Require().NoError(err)

				req = &types.QueryConsensusStateRequest{
					ClientId:     path.EndpointA.ClientID,
					LatestHeight: true,
				}
			},
			true,
		},
		{
			"success with height",
			func() {
				path := ibctesting.NewPath(suite.chainA, suite.chainB)
				path.SetupClients()
				height := path.EndpointA.GetClientState().GetLatestHeight()
				cs := path.EndpointA.GetConsensusState(height)

				var err error
				expConsensusState, err = types.PackConsensusState(cs)
				suite.Require().NoError(err)

				// update client to new height
				err = path.EndpointA.UpdateClient()
				suite.Require().NoError(err)

				req = &types.QueryConsensusStateRequest{
					ClientId:       path.EndpointA.ClientID,
					RevisionNumber: height.GetRevisionNumber(),
					RevisionHeight: height.GetRevisionHeight(),
				}
			},
			true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			suite.SetupTest() // reset

			tc.malleate()
			ctx := suite.chainA.GetContext()
			res, err := suite.chainA.QueryServer.ConsensusState(ctx, req)

			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().NotNil(res)
				suite.Require().Equal(expConsensusState, res.ConsensusState)

				// ensure UnpackInterfaces is defined
				cachedValue := res.ConsensusState.GetCachedValue()
				suite.Require().NotNil(cachedValue)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestQueryConsensusStates() {
	var (
		req                *types.QueryConsensusStatesRequest
		expConsensusStates []types.ConsensusStateWithHeight
	)

	testCases := []struct {
		msg      string
		malleate func()
		expPass  bool
	}{
		{
			"success: without pagination",
			func() {
				req = &types.QueryConsensusStatesRequest{
					ClientId: testClientID,
				}
			},
			true,
		},
		{
			"success, no results",
			func() {
				req = &types.QueryConsensusStatesRequest{
					ClientId: testClientID,
					Pagination: &query.PageRequest{
						Limit:      3,
						CountTotal: true,
					},
				}
			},
			true,
		},
		{
			"success",
			func() {
				path := ibctesting.NewPath(suite.chainA, suite.chainB)
				path.SetupClients()

				height1 := path.EndpointA.GetClientState().GetLatestHeight().(types.Height)
				expConsensusStates = append(
					expConsensusStates,
					types.NewConsensusStateWithHeight(
						height1,
						path.EndpointA.GetConsensusState(height1),
					))

				err := path.EndpointA.UpdateClient()
				suite.Require().NoError(err)

				height2 := path.EndpointA.GetClientState().GetLatestHeight().(types.Height)
				expConsensusStates = append(
					expConsensusStates,
					types.NewConsensusStateWithHeight(
						height2,
						path.EndpointA.GetConsensusState(height2),
					))

				req = &types.QueryConsensusStatesRequest{
					ClientId: path.EndpointA.ClientID,
					Pagination: &query.PageRequest{
						Limit:      3,
						CountTotal: true,
					},
				}
			},
			true,
		},
		{
			"invalid client identifier",
			func() {
				req = &types.QueryConsensusStatesRequest{}
			},
			false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			suite.SetupTest() // reset

			tc.malleate()
			ctx := suite.chainA.GetContext()
			res, err := suite.chainA.QueryServer.ConsensusStates(ctx, req)

			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().NotNil(res)
				suite.Require().Equal(len(expConsensusStates), len(res.ConsensusStates))
				for i := range expConsensusStates {
					suite.Require().NotNil(res.ConsensusStates[i])
					suite.Require().Equal(expConsensusStates[i], res.ConsensusStates[i])
					// ensure UnpackInterfaces is defined
					cachedValue := res.ConsensusStates[i].ConsensusState.GetCachedValue()
					suite.Require().NotNil(cachedValue)
				}
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestQueryConsensusStateHeights() {
	var (
		req                      *types.QueryConsensusStateHeightsRequest
		expConsensusStateHeights []types.Height
	)

	testCases := []struct {
		msg      string
		malleate func()
		expPass  bool
	}{
		{
			"success: without pagination",
			func() {
				req = &types.QueryConsensusStateHeightsRequest{
					ClientId: testClientID,
				}
			},
			true,
		},
		{
			"success: response contains no results",
			func() {
				req = &types.QueryConsensusStateHeightsRequest{
					ClientId: testClientID,
					Pagination: &query.PageRequest{
						Limit:      3,
						CountTotal: true,
					},
				}
			},
			true,
		},
		{
			"success: returns consensus heights",
			func() {
				path := ibctesting.NewPath(suite.chainA, suite.chainB)
				path.SetupClients()

				expConsensusStateHeights = append(expConsensusStateHeights, path.EndpointA.GetClientState().GetLatestHeight().(types.Height))

				err := path.EndpointA.UpdateClient()
				suite.Require().NoError(err)

				expConsensusStateHeights = append(expConsensusStateHeights, path.EndpointA.GetClientState().GetLatestHeight().(types.Height))

				req = &types.QueryConsensusStateHeightsRequest{
					ClientId: path.EndpointA.ClientID,
					Pagination: &query.PageRequest{
						Limit:      3,
						CountTotal: true,
					},
				}
			},
			true,
		},
		{
			"invalid client identifier",
			func() {
				req = &types.QueryConsensusStateHeightsRequest{}
			},
			false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			suite.SetupTest() // reset

			tc.malleate()
			ctx := suite.chainA.GetContext()
			res, err := suite.chainA.QueryServer.ConsensusStateHeights(ctx, req)

			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().NotNil(res)
				suite.Require().Equal(len(expConsensusStateHeights), len(res.ConsensusStateHeights))
				for i := range expConsensusStateHeights {
					suite.Require().NotNil(res.ConsensusStateHeights[i])
					suite.Require().Equal(expConsensusStateHeights[i], res.ConsensusStateHeights[i])
				}
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestQueryClientStatus() {
	var req *types.QueryClientStatusRequest

	testCases := []struct {
		msg       string
		malleate  func()
		expPass   bool
		expStatus string
	}{
		{
			"req is nil",
			func() {
				req = nil
			},
			false, "",
		},
		{
			"invalid clientID",
			func() {
				req = &types.QueryClientStatusRequest{}
			},
			false, "",
		},
		{
			"client not found",
			func() {
				req = &types.QueryClientStatusRequest{
					ClientId: ibctesting.InvalidID,
				}
			},
			false, "",
		},
		{
			"Active client status",
			func() {
				path := ibctesting.NewPath(suite.chainA, suite.chainB)
				path.SetupClients()
				req = &types.QueryClientStatusRequest{
					ClientId: path.EndpointA.ClientID,
				}
			},
			true, exported.Active.String(),
		},
		{
			"Unknown client status",
			func() {
				path := ibctesting.NewPath(suite.chainA, suite.chainB)
				path.SetupClients()
				clientState := path.EndpointA.GetClientState().(*ibctm.ClientState)

				// increment latest height so no consensus state is stored
				clientState.LatestHeight = clientState.LatestHeight.Increment().(types.Height)
				path.EndpointA.SetClientState(clientState)

				req = &types.QueryClientStatusRequest{
					ClientId: path.EndpointA.ClientID,
				}
			},
			true, exported.Expired.String(),
		},
		{
			"Frozen client status",
			func() {
				path := ibctesting.NewPath(suite.chainA, suite.chainB)
				path.SetupClients()
				clientState := path.EndpointA.GetClientState().(*ibctm.ClientState)

				clientState.FrozenHeight = types.NewHeight(0, 1)
				path.EndpointA.SetClientState(clientState)

				req = &types.QueryClientStatusRequest{
					ClientId: path.EndpointA.ClientID,
				}
			},
			true, exported.Frozen.String(),
		},
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			suite.SetupTest() // reset

			tc.malleate()
			ctx := suite.chainA.GetContext()
			res, err := suite.chainA.QueryServer.ClientStatus(ctx, req)

			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().NotNil(res)
				suite.Require().Equal(tc.expStatus, res.Status)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestQueryUpgradedClientState() {
	var (
		req            *types.QueryUpgradedClientStateRequest
		path           *ibctesting.Path
		expClientState *ibctm.ClientState
	)

	upgradePlan := upgradetypes.Plan{
		Name:   "upgrade IBC clients",
		Height: 1000,
	}

	testCases := []struct {
		msg      string
		malleate func()
		expError error
	}{
		{
			"success",
			func() {
				validAuthority := suite.chainA.App.GetIBCKeeper().GetAuthority()

				// update trusting period
				clientState := path.EndpointA.GetClientState()
				clientState.(*ibctm.ClientState).TrustingPeriod += 100

				msg, err := types.NewMsgIBCSoftwareUpgrade(
					validAuthority,
					upgradePlan,
					clientState,
				)
				suite.Require().NoError(err)

				resp, err := suite.chainA.App.GetIBCKeeper().IBCSoftwareUpgrade(suite.chainA.GetContext(), msg)
				suite.Require().NoError(err)
				suite.Require().NotNil(resp)

				expClientState = clientState.(*ibctm.ClientState)
			},
			nil,
		},
		{
			"req is nil",
			func() {
				req = nil
			},
			status.Error(codes.InvalidArgument, "empty request"),
		},
		{
			"no plan",
			func() {
				req = &types.QueryUpgradedClientStateRequest{}
			},
			status.Error(codes.NotFound, "upgrade plan not found"),
		},
		{
			"no upgraded client set in store",
			func() {
				err := suite.chainA.GetSimApp().UpgradeKeeper.ScheduleUpgrade(suite.chainA.GetContext(), upgradePlan)
				suite.Require().NoError(err)
			},
			status.Error(codes.NotFound, "upgraded client not found"),
		},
		{
			"invalid upgraded client state",
			func() {
				err := suite.chainA.GetSimApp().UpgradeKeeper.ScheduleUpgrade(suite.chainA.GetContext(), upgradePlan)
				suite.Require().NoError(err)

				bz := []byte{1, 2, 3}
				err = suite.chainA.GetSimApp().UpgradeKeeper.SetUpgradedClient(suite.chainA.GetContext(), upgradePlan.Height, bz)
				suite.Require().NoError(err)
			},
			status.Error(codes.Internal, "proto: Any: illegal tag 0 (wire type 1)"),
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			suite.SetupTest() // reset

			path = ibctesting.NewPath(suite.chainA, suite.chainB)
			path.SetupClients()

			req = &types.QueryUpgradedClientStateRequest{}

			tc.malleate()

			res, err := suite.chainA.App.GetIBCKeeper().ClientKeeper.UpgradedClientState(suite.chainA.GetContext(), req)

			expPass := tc.expError == nil
			if expPass {
				suite.Require().NoError(err)

				upgradedClientState, err := types.UnpackClientState(res.UpgradedClientState)
				suite.Require().NoError(err)
				upgradedClientStateCmt, ok := upgradedClientState.(*ibctm.ClientState)
				suite.Require().True(ok)

				suite.Require().Equal(expClientState.ZeroCustomFields(), upgradedClientStateCmt)
			} else {
				suite.Require().ErrorIs(err, tc.expError)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestQueryUpgradedConsensusStates() {
	var (
		req               *types.QueryUpgradedConsensusStateRequest
		expConsensusState *codectypes.Any
		height            int64
	)

	testCases := []struct {
		msg      string
		malleate func()
		expPass  bool
	}{
		{
			"req is nil",
			func() {
				req = nil
			},
			false,
		},
		{
			"no plan",
			func() {
				req = &types.QueryUpgradedConsensusStateRequest{}
			},
			false,
		},
		{
			"valid consensus state",
			func() {
				req = &types.QueryUpgradedConsensusStateRequest{}
				lastHeight := types.NewHeight(0, uint64(suite.ctx.BlockHeight()))
				height = int64(lastHeight.GetRevisionHeight())
				suite.ctx = suite.ctx.WithBlockHeight(height)

				expConsensusState = types.MustPackConsensusState(suite.consensusState)
				bz := types.MustMarshalConsensusState(suite.cdc, suite.consensusState)
				err := suite.keeper.SetUpgradedConsensusState(suite.ctx, height, bz)
				suite.Require().NoError(err)
			},
			true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			suite.SetupTest() // reset

			tc.malleate()

			res, err := suite.keeper.UpgradedConsensusState(suite.ctx, req)
			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().True(expConsensusState.Equal(res.UpgradedConsensusState))
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestQueryClientParams() {
	ctx := suite.chainA.GetContext()
	expParams := types.DefaultParams()
	res, _ := suite.chainA.QueryServer.ClientParams(ctx, &types.QueryClientParamsRequest{})
	suite.Require().Equal(&expParams, res.Params)
}

func (suite *KeeperTestSuite) TestQueryVerifyMembershipProof() {
	var (
		path *ibctesting.Path
		req  *types.QueryVerifyMembershipRequest
	)

	testCases := []struct {
		name     string
		malleate func()
		expError error
	}{
		{
			"success",
			func() {
				channel := path.EndpointB.GetChannel()
				bz, err := suite.chainB.Codec.Marshal(&channel)
				suite.Require().NoError(err)

				channelProof, proofHeight := path.EndpointB.QueryProof(host.ChannelKey(path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID))

				merklePath := commitmenttypes.NewMerklePath(host.ChannelPath(path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID))
				merklePath, err = commitmenttypes.ApplyPrefix(suite.chainB.GetPrefix(), merklePath)
				suite.Require().NoError(err)

				req = &types.QueryVerifyMembershipRequest{
					ClientId:    path.EndpointA.ClientID,
					Proof:       channelProof,
					ProofHeight: proofHeight,
					MerklePath:  merklePath,
					Value:       bz,
				}
			},
			nil,
		},
		{
			"req is nil",
			func() {
				req = nil
			},
			errors.New("empty request"),
		},
		{
			"invalid client ID",
			func() {
				req = &types.QueryVerifyMembershipRequest{
					ClientId: "//invalid_id",
				}
			},
			host.ErrInvalidID,
		},
		{
			"empty proof",
			func() {
				req = &types.QueryVerifyMembershipRequest{
					ClientId: ibctesting.FirstClientID,
					Proof:    []byte{},
				}
			},
			errors.New("empty proof"),
		},
		{
			"invalid proof height",
			func() {
				req = &types.QueryVerifyMembershipRequest{
					ClientId:    ibctesting.FirstClientID,
					Proof:       []byte{0x01},
					ProofHeight: types.ZeroHeight(),
				}
			},
			errors.New("proof height must be non-zero"),
		},
		{
			"empty merkle path",
			func() {
				req = &types.QueryVerifyMembershipRequest{
					ClientId:    ibctesting.FirstClientID,
					Proof:       []byte{0x01},
					ProofHeight: types.NewHeight(1, 100),
				}
			},
			errors.New("empty merkle path"),
		},
		{
			"empty value",
			func() {
				req = &types.QueryVerifyMembershipRequest{
					ClientId:    ibctesting.FirstClientID,
					Proof:       []byte{0x01},
					ProofHeight: types.NewHeight(1, 100),
					MerklePath:  commitmenttypes.NewMerklePath("/ibc", host.ChannelPath(mock.PortID, ibctesting.FirstChannelID)),
				}
			},
			errors.New("empty value"),
		},
		{
			"client not found",
			func() {
				req = &types.QueryVerifyMembershipRequest{
					ClientId:    types.FormatClientIdentifier(exported.Tendermint, 100), // use a sequence which hasn't been created yet
					Proof:       []byte{0x01},
					ProofHeight: types.NewHeight(1, 100),
					MerklePath:  commitmenttypes.NewMerklePath("/ibc", host.ChannelPath(mock.PortID, ibctesting.FirstChannelID)),
					Value:       []byte{0x01},
				}
			},
			types.ErrClientNotFound,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset

			path = ibctesting.NewPath(suite.chainA, suite.chainB)
			path.Setup()

			tc.malleate()

			ctx := suite.chainA.GetContext()
			initialGas := ctx.GasMeter().GasConsumed()
			res, err := suite.chainA.QueryServer.VerifyMembership(ctx, req)

			expPass := tc.expError == nil
			if expPass {
				suite.Require().NoError(err)
				suite.Require().True(res.Success, "failed to verify membership proof")

				gasConsumed := ctx.GasMeter().GasConsumed()
				suite.Require().Greater(gasConsumed, initialGas, "gas consumed should be greater than initial gas")
			} else {
				suite.Require().ErrorContains(err, tc.expError.Error())

				gasConsumed := ctx.GasMeter().GasConsumed()
				suite.Require().GreaterOrEqual(gasConsumed, initialGas, "gas consumed should be greater than or equal to initial gas")
			}
		})
	}
}
