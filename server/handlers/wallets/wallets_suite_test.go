package wallets

import (
	"testing"

	"encoding/json"
	"git.zam.io/wallet-backend/wallet-api/models"
	"git.zam.io/wallet-backend/wallet-api/services/wallets"
	"git.zam.io/wallet-backend/wallet-api/services/wallets/mocks"
	"git.zam.io/wallet-backend/web-api/db"
	. "git.zam.io/wallet-backend/web-api/fixtures"
	"git.zam.io/wallet-backend/web-api/fixtures/database"
	"git.zam.io/wallet-backend/web-api/fixtures/database/migrations"
	"git.zam.io/wallet-backend/web-api/server/handlers/base"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

var _ = Describe("testings /wallets endpoints", func() {
	Init()
	database.Init()
	migrations.Init()

	Context("when creating wallet", func() {
		BeforeEachCProvide(func() (*mocks.IGenerator, wallets.IGenerator) {
			g := &mocks.IGenerator{}
			return g, g
		})

		BeforeEachCProvide(func(d *db.Db, generator wallets.IGenerator) base.HandlerFunc {
			return CreateFactory(d, generator)
		})

		ItD("when wallet created successfully", func(handler base.HandlerFunc, d *db.Db, generator *mocks.IGenerator) {
			generatedAddress := "btcaddress"
			generator.On("Create", "btc", "100_btc").Return("", generatedAddress, nil)

			resp, code, err := handler(createContextWithUserID(map[string]interface{}{
				"coin":        "btc",
				"wallet_name": "test wallet",
			}, 100))

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
			_, err = models.GetWallet(d, 100, wID)
			Expect(err).NotTo(HaveOccurred())
		})

		ItD("when no such coin exists", func(handler base.HandlerFunc, generator *mocks.IGenerator) {
			generator.On("Create", "NOTVALIDCOIN", "100_NOTVALIDCOIN").Return("", "", nil)

			resp, _, err := handler(createContextWithUserID(map[string]interface{}{
				"coin":        "NOTVALIDCOIN",
				"wallet_name": "test wallet",
			}, 100))
			Expect(err).To(HaveOccurred())
			Expect(resp).To(BeNil())
			Expect(err).To(Equal(
				base.NewErrorsView("").AddField("body", "coin", "invalid coin"),
			))
		})
	})

	Context("when getting wallets", func() {
		BeforeEachCProvide(func(d *db.Db) (btcWIDs []int64, ethWIDs []int64) {
			// create btc wallets
			for i := 0; i < 5; i++ {
				w, e := models.CreateWallet(d, models.Wallet{Coin: models.Coin{ShortName: "BTC"}})
				Expect(e).NotTo(HaveOccurred())
				btcWIDs = append(btcWIDs, w.ID)
			}

			// create eth wallets
			for i := 0; i < 5; i++ {
				w, e := models.CreateWallet(d, models.Wallet{Coin: models.Coin{ShortName: "ETH"}})
				Expect(e).NotTo(HaveOccurred())
				ethWIDs = append(ethWIDs, w.ID)
			}
			return
		})

		Context("when querying multiple wallets", func() {
			BeforeEachCProvide(func(d *db.Db) base.HandlerFunc {
				return GetAllFactory(d)
			})

			ItD("", func() {

			})
		})
	})
})
