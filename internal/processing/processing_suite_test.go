package processing_test

import (
	"testing"

	"context"
	"git.zam.io/wallet-backend/wallet-api/internal/helpers"
	helpersmock "git.zam.io/wallet-backend/wallet-api/internal/helpers/mocks"
	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"git.zam.io/wallet-backend/wallet-api/internal/services/isc"
	iscmocks "git.zam.io/wallet-backend/wallet-api/internal/services/isc/mocks"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/queries"
	"git.zam.io/wallet-backend/web-api/db"
	. "git.zam.io/wallet-backend/web-api/fixtures"
	"git.zam.io/wallet-backend/web-api/fixtures/database"
	"git.zam.io/wallet-backend/web-api/fixtures/database/migrations"
	"github.com/ericlagergren/decimal"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
)

func TestProcessing(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Processing Suite")
}

const (
	senderPhone      = "+79109998877"
	senderAddress    = "sender addr"
	recipientPhone   = "+79101112233"
	recipientAddress = "recipient addr"

	testCoinName     = "TEST"
	testCoinFullName = "Testing"
)

var _ = Describe("testing txs processing", func() {
	Init()
	database.Init()
	migrations.Init()

	// TODO move it to the fixtures package
	BeforeEachCProvide(func(d *db.Db) (*gorm.DB, error) {
		return gorm.Open("postgres", d.DB.DB)
	})

	BeforeEachCProvide(func() (helpers.IBalance, *helpersmock.IBalance) {
		h := &helpersmock.IBalance{}
		return h, h
	})

	BeforeEachCProvide(func() (isc.ITxsEventNotificator, *iscmocks.ITxsEventNotificator) {
		n := &iscmocks.ITxsEventNotificator{}
		return n, n
	})

	BeforeEachCProvide(func(
		d *gorm.DB,
		balanceHelper helpers.IBalance,
		notificator isc.ITxsEventNotificator,
	) processing.IApi {
		return processing.New(d, balanceHelper, notificator)
	})

	// provide test coin
	BeforeEachCInvoke(func(d *db.Db) {
		_, e := d.Exec(
			"insert into coins (name, short_name, enabled) values ($1, $2, true)",
			testCoinFullName,
			testCoinName,
		)
		Expect(e).NotTo(HaveOccurred())
	})

	Context("when tx is internal and recipient wallet exists", func() {
		type sourceWallet queries.Wallet
		type recipientWallet queries.Wallet

		BeforeEachCProvide(func(d *db.Db) recipientWallet {
			w, err := queries.CreateWallet(d, queries.Wallet{
				UserPhone: recipientPhone,
				Address:   recipientAddress,
				Coin:      queries.Coin{ShortName: testCoinName},
			})
			Expect(err).NotTo(HaveOccurred())
			return recipientWallet(w)
		})

		BeforeEachCProvide(func(d *db.Db) sourceWallet {
			w, err := queries.CreateWallet(d, queries.Wallet{
				UserPhone: senderPhone,
				Address:   senderAddress,
				Coin:      queries.Coin{ShortName: testCoinName},
			})
			Expect(err).NotTo(HaveOccurred())
			return sourceWallet(w)
		})

		Context("when tx amount is wrong", func() {
			ItD(
				"should reject negative amount and not create db record",
				func(d *gorm.DB, s sourceWallet, r recipientWallet, p processing.IApi) {
					tx, err := p.SendInternal(
						context.Background(),
						(*queries.Wallet)(&s),
						processing.NewWalletRecipient((*queries.Wallet)(&r)),
						new(decimal.Big).SetFloat64(-10),
					)
					Expect(tx).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(processing.ErrNegativeAmount))

					By("ensuring db record haven't been created")
					var txs []processing.Tx
					err = d.Model(&processing.Tx{}).Find(&txs).Error
					Expect(err).NotTo(HaveOccurred())
					Expect(len(txs)).To(Equal(0))
				},
			)

			ItD(
				"should reject zero amount and not create db record",
				func(d *gorm.DB, s sourceWallet, r recipientWallet, p processing.IApi) {
					tx, err := p.SendInternal(
						context.Background(),
						(*queries.Wallet)(&s),
						processing.NewWalletRecipient((*queries.Wallet)(&r)),
						new(decimal.Big).SetFloat64(0),
					)
					Expect(tx).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(processing.ErrZeroAmount))

					By("ensuring db record haven't been created")
					var txs []processing.Tx
					err = d.Model(&processing.Tx{}).Find(&txs).Error
					Expect(err).NotTo(HaveOccurred())
					Expect(len(txs)).To(Equal(0))
				},
			)
		})

		Context("when testing balance checks (where Bn is account balance, Bw is user wallet balance, Tx - tx amount)", func() {
			for _, t := range []struct {
				label string
				Bn    float64
				Bw    float64
				Tx    float64
				Err   interface{}
			}{
				{
					"should reject transaction due to Tx > Bn and Tx > Bw", 5, 5, 100,
					[]error{processing.ErrTxAmountToBig, processing.ErrInsufficientFunds},
				},
				{
					"should reject transaction due to Bw > Bn (invalid wallet)", 5, 15, 1,
					processing.ErrInvalidWalletBalance,
				},

				{
					"should reject transaction due to Bw > Bn (invalid wallet) with insufficient funds", 5, 15, 100,
					[]error{processing.ErrTxAmountToBig, processing.ErrInsufficientFunds, processing.ErrInvalidWalletBalance},
				},
			} {
				func(Bn, Bw, Tx float64, expectErr interface{}) {
					ItD(
						t.label,
						func(bmock *helpersmock.IBalance, s sourceWallet, r recipientWallet, p processing.IApi) {
							bmock.On(
								"AccountBalanceCtx",
								mock.Anything,
								testCoinName,
							).Return(new(decimal.Big).SetFloat64(Bn), nil)
							bmock.On(
								"TotalWalletBalanceCtx",
								mock.Anything,
								(*queries.Wallet)(&s),
							).Return(new(decimal.Big).SetFloat64(Bw), nil)

							tx, err := p.SendInternal(
								context.Background(),
								(*queries.Wallet)(&s),
								processing.NewWalletRecipient((*queries.Wallet)(&r)),
								new(decimal.Big).SetFloat64(Tx),
							)
							Expect(err).To(HaveOccurred())
							Expect(err).To(BeEquivalentTo(expectErr))
							Expect(tx).NotTo(BeNil())
							Expect(tx.Status).NotTo(BeNil())
							Expect(tx.Status.Name).To(Equal("decline"))
						},
					)
				}(t.Bn, t.Bw, t.Tx, t.Err)
			}
		})
	})

	Context("when testing money flow", func() {
		ItD("should transfer from A to B, general balance should remain unchanged", func() {

		})
	})
})
