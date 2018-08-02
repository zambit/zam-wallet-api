package wallets

import (
	"testing"

	"encoding/json"
	"fmt"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes/mocks"
	"git.zam.io/wallet-backend/wallet-api/pkg/models"
	"git.zam.io/wallet-backend/web-api/db"
	. "git.zam.io/wallet-backend/web-api/fixtures"
	"git.zam.io/wallet-backend/web-api/fixtures/database"
	"git.zam.io/wallet-backend/web-api/fixtures/database/migrations"
	"git.zam.io/wallet-backend/web-api/server/handlers/base"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"net/http"
	"strconv"
	"strings"
)

func TestWallets(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Wallets Suite")
}

func createContextWithUserID(body interface{}, userID int64) *gin.Context {
	bytes, err := json.Marshal(body)
	Expect(err).NotTo(HaveOccurred())
	req, _ := http.NewRequest("POST", "nomatter", strings.NewReader(string(bytes)))

	c := &gin.Context{
		Request: req,
	}
	c.Set("user_id", userID)
	return c
}

func createContextWithQueryParams(userID int64, params ...interface{}) *gin.Context {
	if len(params)%2 != 0 {
		panic("odd count of params is required")
	}

	queryParts := make([]string, 0, len(params)/2)
	for i := 0; i < len(params); i += 2 {
		queryParts = append(queryParts, fmt.Sprintf("%s=%v", params[i].(string), params[i+1]))
	}
	query := ""
	if len(queryParts) > 0 {
		query = "?" + strings.Join(queryParts, "&")
	}

	req, _ := http.NewRequest("GET", "nomatter"+query, nil)

	c := &gin.Context{
		Request: req,
	}
	c.Set("user_id", userID)
	return c
}

var _ = Describe("testings /wallets endpoints", func() {
	Init()
	database.Init()
	migrations.Init()

	Context("when creating wallet", func() {
		const (
			userID = 100
		)

		BeforeEachCProvide(func() (*mocks.IGenerator, nodes.IGenerator) {
			g := &mocks.IGenerator{}
			return g, g
		})
		BeforeEachCProvide(func(generator *mocks.IGenerator) (*mocks.ICoordinator, nodes.ICoordinator) {
			c := &mocks.ICoordinator{}
			c.On("Generator", mock.Anything).Return(generator, nil)
			return c, c
		})

		BeforeEachCProvide(func(d *db.Db, coordinator nodes.ICoordinator) base.HandlerFunc {
			return CreateFactory(d, coordinator)
		})

		ItD("should create wallet successfully", func(handler base.HandlerFunc, d *db.Db, generator *mocks.IGenerator) {
			generatedAddress := "btcaddress"
			generator.On("Create").Return(generatedAddress, nil)

			resp, code, err := handler(createContextWithUserID(map[string]interface{}{
				"name":        "btc",
				"wallet_name": "test wallet",
			}, userID))

			Expect(err).NotTo(HaveOccurred())
			Expect(code).To(Equal(201))
			Expect(resp).NotTo(BeNil())
			Expect(resp).To(BeAssignableToTypeOf(Response{}))

			walletResponse := resp.(Response).Wallet

			Expect(walletResponse.Name).NotTo(Equal("test wallet"), "passed wallet name should be ignored so far")
			Expect(walletResponse.Name).To(Equal("BTC wallet"))
			Expect(walletResponse.Coin).To(Equal("btc"))
			Expect(walletResponse.Address).To(Equal(generatedAddress))

			By("ensuring db state")
			wID, err := strconv.ParseInt(walletResponse.ID, 10, 64)
			Expect(err).NotTo(HaveOccurred())
			_, err = models.GetWallet(d, userID, wID)
			Expect(err).NotTo(HaveOccurred())
		})

		ItD("should return 'no such name' error ", func(handler base.HandlerFunc, generator *mocks.IGenerator) {
			generator.On("Create", "NOTVALIDCOIN", "100_NOTVALIDCOIN").Return("", "", nil)

			resp, _, err := handler(createContextWithUserID(map[string]interface{}{
				"name":        "NOTVALIDCOIN",
				"wallet_name": "test wallet",
			}, userID))
			Expect(err).To(HaveOccurred())
			Expect(resp).To(BeNil())
			Expect(err).To(Equal(
				base.NewErrorsView("").AddField("body", "name", "invalid name"),
			))
		})

		ItD("should reject wallet creation due to wallet duplication", func(d *db.Db, handler base.HandlerFunc) {
			By("manually creating first wallet")
			_, err := models.CreateWallet(d, models.Wallet{
				UserID: userID,
				Coin: models.Coin{
					ShortName: "btc",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("performing query")
			resp, _, err := handler(createContextWithUserID(map[string]interface{}{
				"name": "btc",
			}, userID))
			Expect(err).To(HaveOccurred())
			Expect(resp).To(BeNil())
			Expect(err).To(Equal(
				base.NewErrorsView("").AddField("body", "coin", "wallet of such coin already exists"),
			))
		})
	})

	idToView := func(id int64) string {
		return fmt.Sprint(id)
	}

	Context("when getting wallets", func() {
		var userID int64 = 10
		type btcWIDsT []string
		type ethWIDsT []string
		BeforeEachCProvide(func(d *db.Db) (btcWIDs btcWIDsT, ethWIDs ethWIDsT) {
			// drop unique user/coin constraint
			_, err := d.Exec("alter table wallets drop constraint wallets_unique_user_coin_pair_cst;")
			Expect(err).NotTo(HaveOccurred())

			_, err = d.Exec("update coins set enabled = true where short_name = 'ETH'")
			Expect(err).NotTo(HaveOccurred())

			// create btc wallets
			for i := 0; i < 5; i++ {
				w, e := models.CreateWallet(d, models.Wallet{
					UserID: userID,
					Coin:   models.Coin{ShortName: "BTC"}},
				)
				Expect(e).NotTo(HaveOccurred())
				btcWIDs = append(btcWIDs, idToView(w.ID))
			}

			// create eth wallets
			for i := 0; i < 5; i++ {
				w, e := models.CreateWallet(d, models.Wallet{
					UserID: userID,
					Coin:   models.Coin{ShortName: "ETH"}},
				)
				Expect(e).NotTo(HaveOccurred())
				ethWIDs = append(ethWIDs, idToView(w.ID))
			}

			// create constraint again preventing migrations failure
			_, err = d.Exec("alter table wallets add constraint wallets_unique_user_coin_pair_cst unique (id);")
			Expect(err).NotTo(HaveOccurred())

			return
		})

		Context("when querying multiple wallets", func() {
			BeforeEachCProvide(func(d *db.Db) base.HandlerFunc {
				return GetAllFactory(d)
			})

			ItD("should return all rows due to no filters", func(handler base.HandlerFunc, btcWIDs btcWIDsT, ethWIDs ethWIDsT) {
				resp, _, err := handler(createContextWithQueryParams(userID))
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).To(BeAssignableToTypeOf(AllResponse{}))

				respView := resp.(AllResponse)
				Expect(respView.Count).To(BeEquivalentTo(len(btcWIDs) + len(ethWIDs)))

				for i, id := range btcWIDs {
					Expect(id).To(Equal(respView.Wallets[i].ID))
				}

				for i, id := range ethWIDs {
					i2 := i + len(btcWIDs)
					Expect(id).To(Equal(respView.Wallets[i2].ID))
				}
			})

			ItD(
				"should return only BTC wallets",
				func(handler base.HandlerFunc, btcWIDs btcWIDsT) {
					resp, _, err := handler(createContextWithQueryParams(userID, "coin", "BTC"))
					Expect(err).NotTo(HaveOccurred())
					Expect(resp).To(BeAssignableToTypeOf(AllResponse{}))

					respView := resp.(AllResponse)
					Expect(respView.Count).To(BeEquivalentTo(len(btcWIDs)))

					for i, id := range btcWIDs {
						Expect(id).To(Equal(respView.Wallets[i].ID))
					}
				},
			)

			ItD(
				"should return only ETH wallets",
				func(handler base.HandlerFunc, ethWIDs ethWIDsT) {
					resp, _, err := handler(createContextWithQueryParams(userID, "coin", "ETH"))
					Expect(err).NotTo(HaveOccurred())
					Expect(resp).To(BeAssignableToTypeOf(AllResponse{}))

					respView := resp.(AllResponse)
					Expect(respView.Count).To(BeEquivalentTo(len(ethWIDs)))

					for i, id := range ethWIDs {
						Expect(id).To(Equal(respView.Wallets[i].ID))
					}
				},
			)
		})
	})
})
