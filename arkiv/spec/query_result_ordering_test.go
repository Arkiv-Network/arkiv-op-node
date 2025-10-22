package spec_test

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/golem-base/storagetx"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Query Result Ordering", func() {

	BeforeEach(
		func(ctx context.Context) {
			entities := QueryResponse{}
			err := client.Client().CallContext(
				ctx,
				&entities,
				"arkiv_query",
				"$owner="+walletAddress.Hex(),
			)
			Expect(err).To(Not(HaveOccurred()), "failed to get entities")

			for _, entity := range entities.Data {
				receipt := submitAndWaitForStorageTransaction(ctx, storagetx.StorageTransaction{
					Delete: []common.Hash{entity.Key},
				})
				Expect(receipt.Status).To(Equal(types.ReceiptStatusSuccessful), "transaction failed")
			}

		},
	)

	When("I create 20 entities in the same transaction", func() {

		var entityIDs []common.Hash

		BeforeEach(func(ctx context.Context) {

			entityIDs = []common.Hash{}

			entities := []EntityData{}
			for i := range 20 {
				entities = append(entities, EntityData{
					Value: []byte(fmt.Sprintf("entity-payload-%d", i)),
					Owner: walletAddress,
					StringAnnotations: map[string]string{
						fmt.Sprintf("string_annotation_%d_1", i): fmt.Sprintf("sa_value_%d_1", i),
						fmt.Sprintf("string_annotation_%d_2", i): fmt.Sprintf("sa_value_%d_2", i),
					},
					NumericAnnotations: map[string]uint64{
						fmt.Sprintf("numeric_annotation_%d_1", i): uint64(i),
						fmt.Sprintf("numeric_annotation_%d_2", i): uint64(i),
					},
				})
			}

			creates := []storagetx.Create{}
			for _, entity := range entities {
				creates = append(creates, entity.ToCreateStorageTransaction(200000))
			}

			receipt := submitAndWaitForStorageTransaction(ctx, storagetx.StorageTransaction{
				Create: creates,
			})

			for _, log := range receipt.Logs {
				if log.Topics[0] == storagetx.GolemBaseStorageEntityCreated {
					entityIDs = append(entityIDs, log.Topics[1])
				}
			}

			Expect(entityIDs).To(HaveLen(len(entities)), "should have 20 entities")

		})
		When("I query the entities by owner", func() {
			It("should return the entities in the correct order", func(ctx context.Context) {
				entities := QueryResponse{}
				err := client.Client().CallContext(
					ctx,
					&entities,
					"arkiv_query",
					"$owner="+walletAddress.Hex(),
				)
				Expect(err).To(Not(HaveOccurred()), "failed to get entities")
				Expect(entities.Data).To(HaveLen(len(entities.Data)), "should have 20 entities")
				for i, entity := range entities.Data {
					Expect(entityIDs[i]).To(Equal(entity.Key))
				}
			})
		})
	})

	When("I create 20 entities in the consecutive transactions", func() {
		var entityIDs []common.Hash

		BeforeEach(func(ctx context.Context) {

			entityIDs = []common.Hash{}

			for i := range 20 {
				entityData := EntityData{
					Value: []byte(fmt.Sprintf("entity-payload-%d", i)),
					Owner: walletAddress,
					StringAnnotations: map[string]string{
						fmt.Sprintf("string_annotation_%d_1", i): fmt.Sprintf("sa_value_%d_1", i),
						fmt.Sprintf("string_annotation_%d_2", i): fmt.Sprintf("sa_value_%d_2", i),
					},
					NumericAnnotations: map[string]uint64{
						fmt.Sprintf("numeric_annotation_%d_1", i): uint64(i),
						fmt.Sprintf("numeric_annotation_%d_2", i): uint64(i),
					},
				}

				receipt := submitAndWaitForStorageTransaction(ctx, storagetx.StorageTransaction{
					Create: []storagetx.Create{entityData.ToCreateStorageTransaction(200000)},
				})

				for _, log := range receipt.Logs {
					if log.Topics[0] == storagetx.GolemBaseStorageEntityCreated {
						entityIDs = append(entityIDs, log.Topics[1])
						break
					}
				}

			}

			Expect(entityIDs).To(HaveLen(20), "should have 20 entities")

		})

		When("I query the entities by owner", func() {
			It("should return the entities in the correct order", func(ctx context.Context) {
				entities := QueryResponse{}
				err := client.Client().CallContext(
					ctx,
					&entities,
					"arkiv_query",
					"$owner="+walletAddress.Hex(),
				)
				Expect(err).To(Not(HaveOccurred()), "failed to get entities")
				Expect(entities.Data).To(HaveLen(len(entities.Data)), "should have 20 entities")
				for i, entity := range entities.Data {
					Expect(entityIDs[i]).To(Equal(entity.Key))
				}
			})
		})

	})

})
