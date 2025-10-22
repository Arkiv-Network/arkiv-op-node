package spec_test

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/golem-base/address"
	"github.com/ethereum/go-ethereum/golem-base/storagetx"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/rlp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type IncludeData struct {
	Key         bool `json:"key"`
	Annotations bool `json:"annotations"`
	Payload     bool `json:"payload"`
	Expiration  bool `json:"expiration"`
	Owner       bool `json:"owner"`
}

type QueryOptions struct {
	AtBlock        *uint64      `json:"atBlock"`
	IncludeData    *IncludeData `json:"includeData"`
	ResultsPerPage uint64       `json:"resultsPerPage"`
	Cursor         string       `json:"cursor"`
}

type EntityDataResponse struct {
	Key       common.Hash    `json:"key"`
	Value     hexutil.Bytes  `json:"value"`
	ExpiresAt uint64         `json:"expiresAt"`
	Owner     common.Address `json:"owner"`

	StringAnnotations  []entity.StringAnnotation  `json:"stringAnnotations"`
	NumericAnnotations []entity.NumericAnnotation `json:"numericAnnotations"`
}

type QueryResponse struct {
	Data        []EntityDataResponse `json:"data"`
	BlockNumber uint64               `json:"blockNumber"`
	Cursor      string               `json:"cursor"`
}

func (q EntityDataResponse) ToEntityData() *EntityData {
	ed := EntityData{
		Key:                q.Key,
		Value:              q.Value,
		ExpiresAt:          q.ExpiresAt,
		Owner:              q.Owner,
		StringAnnotations:  make(map[string]string),
		NumericAnnotations: make(map[string]uint64),
	}
	for _, sa := range q.StringAnnotations {
		ed.StringAnnotations[sa.Key] = sa.Value
	}
	for _, na := range q.NumericAnnotations {
		ed.NumericAnnotations[na.Key] = na.Value
	}
	return &ed
}

type EntityData struct {
	Key                common.Hash       `json:"key"`
	Value              []byte            `json:"value"`
	ExpiresAt          uint64            `json:"expires_at"`
	Owner              common.Address    `json:"owner"`
	StringAnnotations  map[string]string `json:"string_annotations"`
	NumericAnnotations map[string]uint64 `json:"numeric_annotations"`
}

func (ed EntityData) ToCreateStorageTransaction(btl uint64) storagetx.Create {

	cr := storagetx.Create{
		BTL:                btl,
		Payload:            ed.Value,
		StringAnnotations:  make([]entity.StringAnnotation, 0),
		NumericAnnotations: make([]entity.NumericAnnotation, 0),
	}

	for k, v := range ed.StringAnnotations {
		cr.StringAnnotations = append(cr.StringAnnotations, entity.StringAnnotation{
			Key:   k,
			Value: v,
		})
	}
	for k, v := range ed.NumericAnnotations {
		cr.NumericAnnotations = append(cr.NumericAnnotations, entity.NumericAnnotation{
			Key:   k,
			Value: v,
		})
	}
	return cr
}

func (ed EntityData) ToUpdateStorageTransaction(btl uint64) storagetx.Update {

	cr := storagetx.Update{
		EntityKey:          ed.Key,
		BTL:                btl,
		Payload:            ed.Value,
		StringAnnotations:  make([]entity.StringAnnotation, 0, len(ed.StringAnnotations)),
		NumericAnnotations: make([]entity.NumericAnnotation, 0, len(ed.NumericAnnotations)),
	}

	for k, v := range ed.StringAnnotations {
		cr.StringAnnotations = append(cr.StringAnnotations, entity.StringAnnotation{
			Key:   k,
			Value: v,
		})
	}
	for k, v := range ed.NumericAnnotations {
		cr.NumericAnnotations = append(cr.NumericAnnotations, entity.NumericAnnotation{
			Key:   k,
			Value: v,
		})
	}
	return cr
}

func submitAndWaitForStorageTransaction(ctx context.Context, storageTx storagetx.StorageTransaction) *types.Receipt {
	data, err := rlp.EncodeToBytes(&storageTx)
	Expect(err).To(Not(HaveOccurred()), "failed to encode transaction data")

	nonce, err := client.PendingNonceAt(ctx, walletAddress)
	Expect(err).To(Not(HaveOccurred()), "failed to get pending nonce")

	gasPrice, err := client.SuggestGasPrice(ctx)
	Expect(err).To(Not(HaveOccurred()), "failed to suggest gas price")

	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:      walletAddress,
		GasFeeCap: gasPrice,
		To:        &address.GolemBaseStorageProcessorAddress,
		Data:      data,
	})
	Expect(err).To(Not(HaveOccurred()), "failed to estimate gas")

	tx := types.NewTx(
		&types.DynamicFeeTx{
			To:        &address.GolemBaseStorageProcessorAddress,
			ChainID:   chainID,
			Nonce:     nonce,
			Gas:       gas,
			GasFeeCap: gasPrice,
			GasTipCap: big.NewInt(2),
			Data:      data,
		},
	)

	signer := types.NewLondonSigner(chainID)
	tx, err = types.SignTx(tx, signer, pk)
	Expect(err).To(Not(HaveOccurred()), "failed to sign transaction")

	err = client.SendTransaction(ctx, tx)
	Expect(err).To(Not(HaveOccurred()), "failed to send transaction")

	// Wait for transaction to be mined
	receipt, err := waitMined(ctx, client, tx.Hash())
	Expect(err).To(Not(HaveOccurred()), "failed to wait for transaction to be mined")

	Expect(receipt.Status).To(Equal(types.ReceiptStatusSuccessful), "transaction failed")

	return receipt
}

type EntityVersion struct {
	Data        *EntityData
	BlockNumber uint64
}

var _ = Describe("Creation", func() {
	// var entityID common.Hash
	// var createBlock uint64
	var entityVersions []EntityVersion

	When("I create a new entity", func() {
		BeforeEach(func(ctx context.Context) {

			entityData := EntityData{
				Value: []byte("entity-payload-1"),
				Owner: walletAddress,
				StringAnnotations: map[string]string{
					"string_annotation_1_1": "sa_value_1_1",
					"string_annotation_1_2": "sa_value_1_2",
				},
				NumericAnnotations: map[string]uint64{
					"numeric_annotation_1_1": 1,
					"numeric_annotation_1_2": 2,
				},
			}

			create := entityData.ToCreateStorageTransaction(200000)

			receipt := submitAndWaitForStorageTransaction(ctx, storagetx.StorageTransaction{
				Create: []storagetx.Create{
					create,
				},
			})

			Expect(receipt.Status).To(Equal(types.ReceiptStatusSuccessful), "transaction failed")

			entityID := common.Hash{}

			for _, log := range receipt.Logs {
				if log.Topics[0] == storagetx.GolemBaseStorageEntityCreated {
					entityID = log.Topics[1]
					break
				}
			}
			Expect(entityID).To(Not(Equal(common.Hash{})), "entity ID is empty")

			entityData.Key = entityID
			entityData.ExpiresAt = receipt.BlockNumber.Uint64() + 200000

			entityVersions = []EntityVersion{
				{
					Data:        &entityData,
					BlockNumber: receipt.BlockNumber.Uint64(),
				},
			}

		})

		It("should exist", func(ctx context.Context) {

			entities := QueryResponse{}
			err := client.Client().CallContext(
				ctx,
				&entities,
				"arkiv_query",
				"$key="+entityVersions[0].Data.Key.Hex(),
			)
			Expect(err).To(Not(HaveOccurred()), "failed to get entities")
			Expect(entities.Data).To(HaveLen(1), "should have 1 entity")
			Expect(entities.Data[0].ToEntityData()).To(Equal(entityVersions[0].Data))
		})

		It("should be found by string annotation", func(ctx context.Context) {
			entities := QueryResponse{}
			err := client.Client().CallContext(
				ctx, &entities,
				"arkiv_query",
				`string_annotation_1_1="sa_value_1_1" && $key=`+entityVersions[0].Data.Key.Hex(),
			)
			Expect(err).To(Not(HaveOccurred()), "failed to get entities")
			Expect(entities.Data).To(HaveLen(1), "should have 1 entity")
			Expect(entities.Data[0].ToEntityData()).To(Equal(entityVersions[0].Data))
		})

		It("should be found by numeric annotation", func(ctx context.Context) {
			entities := QueryResponse{}
			err := client.Client().CallContext(
				ctx,
				&entities,
				"arkiv_query",
				`numeric_annotation_1_1=1 && $key=`+entityVersions[0].Data.Key.Hex(),
			)
			Expect(err).To(Not(HaveOccurred()), "failed to get entities")
			Expect(entities.Data).To(HaveLen(1), "should have 1 entity")
			Expect(entities.Data[0].ToEntityData()).To(Equal(entityVersions[0].Data))
		})

		When("I update the entity", func() {
			BeforeEach(func(ctx context.Context) {

				entityData := EntityData{
					Key:   entityVersions[0].Data.Key,
					Value: []byte("entity-payload-2"),
					Owner: walletAddress,
					StringAnnotations: map[string]string{
						"string_annotation_2_1": "sa_value_2_1",
						"string_annotation_2_2": "sa_value_2_2",
					},
					NumericAnnotations: map[string]uint64{},
				}

				receipt := submitAndWaitForStorageTransaction(
					ctx,
					storagetx.StorageTransaction{
						Update: []storagetx.Update{
							entityData.ToUpdateStorageTransaction(300000),
						},
					},
				)
				Expect(receipt.Status).To(Equal(types.ReceiptStatusSuccessful), "transaction failed")

				entityData.ExpiresAt = receipt.BlockNumber.Uint64() + 300000
				entityVersions = append(entityVersions, EntityVersion{
					Data:        &entityData,
					BlockNumber: receipt.BlockNumber.Uint64(),
				})
			})

			It("should be found by key", func(ctx context.Context) {
				entities := QueryResponse{}
				err := client.Client().CallContext(
					ctx,
					&entities, "arkiv_query",
					"$key="+entityVersions[0].Data.Key.Hex(),
				)
				Expect(err).To(Not(HaveOccurred()), "failed to get entities")
				Expect(entities.Data).To(HaveLen(1), "should have 1 entity")
				Expect(entities.Data[0].ToEntityData()).To(Equal(entityVersions[1].Data))
			})

			It("should be found by string annotation", func(ctx context.Context) {
				entities := QueryResponse{}
				err := client.Client().CallContext(
					ctx,
					&entities,
					"arkiv_query",
					`string_annotation_2_1="sa_value_2_1" && $key=`+entityVersions[0].Data.Key.Hex(),
				)
				Expect(err).To(Not(HaveOccurred()), "failed to get entities")
				Expect(entities.Data).To(HaveLen(1), "should have 1 entity")
				Expect(entities.Data[0].ToEntityData()).To(Equal(entityVersions[1].Data))
			})
			It("should not be found by the old string annotation", func(ctx context.Context) {
				entities := QueryResponse{}
				err := client.Client().CallContext(
					ctx, &entities,
					"arkiv_query",
					`string_annotation_1_1="sa_value_1_1" && $key=`+entityVersions[0].Data.Key.Hex(),
					map[string]any{},
				)
				Expect(err).To(Not(HaveOccurred()), "failed to get entities")
				Expect(entities.Data).To(HaveLen(0), "should have 0 entities")
			})

			It("should be found by querying the previous block with the old string annotation", func(ctx context.Context) {
				entities := QueryResponse{}
				err := client.Client().CallContext(
					ctx,
					&entities,
					"arkiv_query", `string_annotation_1_1="sa_value_1_1" && $key=`+entityVersions[0].Data.Key.Hex(),
					QueryOptions{
						AtBlock: &entityVersions[0].BlockNumber,
					},
				)
				Expect(err).To(Not(HaveOccurred()), "failed to get entities")
				Expect(entities.Data).To(HaveLen(1), "should have 1 entity")
				Expect(entities.Data[0].ToEntityData()).To(Equal(entityVersions[0].Data))
			})

			When("I update the entity again with a new string annotation", func() {
				BeforeEach(func(ctx context.Context) {

					entityData := EntityData{
						Key:   entityVersions[0].Data.Key,
						Value: []byte("entity-payload-3"),
						Owner: walletAddress,
						StringAnnotations: map[string]string{
							"string_annotation_3_1": "sa_value_3_1",
						},
						NumericAnnotations: map[string]uint64{},
					}

					receipt := submitAndWaitForStorageTransaction(
						ctx,
						storagetx.StorageTransaction{
							Update: []storagetx.Update{
								entityData.ToUpdateStorageTransaction(400000),
							},
						},
					)
					Expect(receipt.Status).To(Equal(types.ReceiptStatusSuccessful), "transaction failed")

					entityData.ExpiresAt = receipt.BlockNumber.Uint64() + 400000
					entityVersions = append(entityVersions, EntityVersion{
						Data:        &entityData,
						BlockNumber: receipt.BlockNumber.Uint64(),
					})

				})

				It("should be found by string annotation", func(ctx context.Context) {
					entities := QueryResponse{}
					err := client.Client().CallContext(
						ctx,
						&entities,
						"arkiv_query",
						`string_annotation_3_1="sa_value_3_1" && $key=`+entityVersions[0].Data.Key.Hex(),
					)
					Expect(err).To(Not(HaveOccurred()), "failed to get entities")
					Expect(entities.Data).To(HaveLen(1), "should have 1 entity")
					Expect(entities.Data[0].ToEntityData()).To(Equal(entityVersions[2].Data))
				})

				It("previous update should be found by the previous update's string annotation and at the previous block", func(ctx context.Context) {
					entities := QueryResponse{}
					err := client.Client().CallContext(
						ctx,
						&entities,
						"arkiv_query",
						`string_annotation_2_1="sa_value_2_1" && $key=`+entityVersions[0].Data.Key.Hex(),
						QueryOptions{
							AtBlock: &entityVersions[1].BlockNumber,
						},
					)
					Expect(err).To(Not(HaveOccurred()), "failed to get entities")
					Expect(entities.Data).To(HaveLen(1), "should have 1 entity")
					Expect(entities.Data[0].ToEntityData()).To(Equal(entityVersions[1].Data))
				})

				It("initail create update should be found by the previous update's string annotation and at the initial block", func(ctx context.Context) {
					entities := QueryResponse{}
					err := client.Client().CallContext(
						ctx,
						&entities,
						"arkiv_query",
						`string_annotation_1_1="sa_value_1_1" && $key=`+entityVersions[0].Data.Key.Hex(),
						QueryOptions{
							AtBlock: &entityVersions[0].BlockNumber,
						},
					)
					Expect(err).To(Not(HaveOccurred()), "failed to get entities")
					Expect(entities.Data).To(HaveLen(1), "should have 1 entity")
					Expect(entities.Data[0].ToEntityData()).To(Equal(entityVersions[0].Data))
				})

				When("I delete the entity", func() {
					BeforeEach(func(ctx context.Context) {
						receipt := submitAndWaitForStorageTransaction(ctx, storagetx.StorageTransaction{
							Delete: []common.Hash{entityVersions[0].Data.Key},
						})
						Expect(receipt.Status).To(Equal(types.ReceiptStatusSuccessful), "transaction failed")
						entityVersions = append(entityVersions, EntityVersion{
							Data:        nil,
							BlockNumber: receipt.BlockNumber.Uint64(),
						})
					})

					It("should not be found by the pre-deletion string annotation", func(ctx context.Context) {
						entities := QueryResponse{}
						err := client.Client().CallContext(
							ctx,
							&entities,
							"arkiv_query",
							`string_annotation_3_1="sa_value_3_1" && $key=`+entityVersions[0].Data.Key.Hex(),
						)
						Expect(err).To(Not(HaveOccurred()), "failed to get entities")
						Expect(entities.Data).To(HaveLen(0), "should have 0 entities")
					})

					It("should not be found by the initial string annotation", func(ctx context.Context) {
						entities := QueryResponse{}
						err := client.Client().CallContext(
							ctx,
							&entities,
							"arkiv_query",
							`string_annotation_1_1="sa_value_1_1" && $key=`+entityVersions[0].Data.Key.Hex(),
						)
						Expect(err).To(Not(HaveOccurred()), "failed to get entities")
						Expect(entities.Data).To(HaveLen(0), "should have 0 entities")
					})

					It("should be found by querying the previous block with the pre-deletion string annotation", func(ctx context.Context) {
						entities := QueryResponse{}
						err := client.Client().CallContext(
							ctx,
							&entities,
							"arkiv_query", `string_annotation_3_1="sa_value_3_1" && $key=`+entityVersions[0].Data.Key.Hex(),
							QueryOptions{
								AtBlock: &entityVersions[2].BlockNumber,
							},
						)
						Expect(err).To(Not(HaveOccurred()), "failed to get entities")
						Expect(entities.Data).To(HaveLen(1), "should have 1 entity")
						Expect(entities.Data[0].ToEntityData()).To(Equal(entityVersions[2].Data))
					})

					It("should be found by querying the previous block with the entity key", func(ctx context.Context) {
						entities := QueryResponse{}
						err := client.Client().CallContext(
							ctx,
							&entities,
							"arkiv_query", `$key=`+entityVersions[0].Data.Key.Hex(),
							QueryOptions{
								AtBlock: &entityVersions[2].BlockNumber,
							},
						)
						Expect(err).To(Not(HaveOccurred()), "failed to get entities")
						Expect(entities.Data).To(HaveLen(1), "should have 1 entity")
						Expect(entities.Data[0].ToEntityData()).To(Equal(entityVersions[2].Data))
					})

				})

			})

		})

	})

})
