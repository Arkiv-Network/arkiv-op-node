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

var _ = Describe("Pagination", func() {

	// delete all entities
	BeforeEach(
		func(ctx context.Context) {
			entities := QueryResponse{}
			err := client.Client().CallContext(
				ctx,
				&entities,
				"arkiv_query",
				"$owner="+walletAddress.Hex(),
				QueryOptions{},
			)
			Expect(err).To(Not(HaveOccurred()), "failed to get entities")

			deletes := []common.Hash{}

			for _, entity := range entities.Data {
				deletes = append(deletes, entity.Key)
			}

			for len(deletes) > 0 {
				chunk := deletes
				if len(deletes) > 500 {
					chunk = deletes[:500]

				}
				receipt := submitAndWaitForStorageTransaction(ctx, storagetx.StorageTransaction{
					Delete: chunk,
				})
				Expect(receipt.Status).To(Equal(types.ReceiptStatusSuccessful), "transaction failed")
				deletes = deletes[len(chunk):]
			}

		},
	)

	When("I create 10 entities", func() {

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

		When("I query the entities by owner and requiest page of 5 entities", func() {

			var response QueryResponse

			BeforeEach(func(ctx context.Context) {
				response = QueryResponse{}
				err := client.Client().CallContext(
					ctx,
					&response,
					"arkiv_query",
					"$owner="+walletAddress.Hex(),
					QueryOptions{
						ResultsPerPage: 5,
					},
				)
				Expect(err).To(Not(HaveOccurred()), "failed to get entities")
			})

			It("should return the entities in the correct order", func(ctx context.Context) {
				Expect(response.Data).To(HaveLen(5), "should have 5 entities")
				Expect(response.BlockNumber).ToNot(Equal(uint64(0)))

				ids := []common.Hash{}
				for _, entity := range response.Data {
					ids = append(ids, entity.Key)
				}
				Expect(ids).To(Equal(entityIDs[:5]), "should have the correct entities")
			})

			When("I request the next page", func() {

				BeforeEach(func(ctx context.Context) {
					offset := response.Cursor
					response = QueryResponse{}

					err := client.Client().CallContext(
						ctx,
						&response,
						"arkiv_query",
						"$owner="+walletAddress.Hex(),
						QueryOptions{
							ResultsPerPage: 5,
							Cursor:         offset,
						},
					)
					Expect(err).To(Not(HaveOccurred()), "failed to get entities")
				})

				It("should return the next page of entities in the correct order", func(ctx context.Context) {
					Expect(response.Data).To(HaveLen(5), "should have 5 entities")
					Expect(response.BlockNumber).ToNot(Equal(uint64(0)))
					// Expect(response.Cursor).To(Equal((10)))

					ids := []common.Hash{}
					for _, entity := range response.Data {
						ids = append(ids, entity.Key)
					}
					Expect(ids).To(Equal(entityIDs[5:10]), "should have the correct entities")
				})

				When("I request the last", func() {

					BeforeEach(func(ctx context.Context) {
						offset := response.Cursor
						response = QueryResponse{}
						err := client.Client().CallContext(
							ctx,
							&response,
							"arkiv_query",
							"$owner="+walletAddress.Hex(),
							QueryOptions{
								Cursor: offset,
							},
						)
						Expect(err).To(Not(HaveOccurred()), "failed to get entities")
					})

					It("should return the next page of entities in the correct order", func(ctx context.Context) {
						Expect(response.Data).To(HaveLen(10), "should have 10 entities")
						Expect(response.BlockNumber).ToNot(Equal(uint64(0)))

						ids := []common.Hash{}
						for _, entity := range response.Data {
							ids = append(ids, entity.Key)
						}
						Expect(ids).To(Equal(entityIDs[10:20]), "should have the correct entities")
					})

				})
			})

		})

	})

})
