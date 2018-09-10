package processing_test

import (
	"testing"

	"context"
	"fmt"
	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"git.zam.io/wallet-backend/wallet-api/internal/services/isc"
	iscmocks "git.zam.io/wallet-backend/wallet-api/internal/services/isc/mocks"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/queries"
	"git.zam.io/wallet-backend/web-api/db"
	. "git.zam.io/wallet-backend/web-api/fixtures"
	"git.zam.io/wallet-backend/web-api/fixtures/database"
	"git.zam.io/wallet-backend/web-api/fixtures/database/migrations"
	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strings"
	"git.zam.io/wallet-backend/wallet-api/internal/helpers/balance"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes/mocks"
	"git.zam.io/wallet-backend/wallet-api/internal/helpers"
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

//
type flowActors map[string]*queries.Wallet

func (actors flowActors) get(index string) *queries.Wallet {
	w, ok := actors[index]
	if !ok {
		panic(fmt.Errorf("no such actor in the flow: %s", index))
	}
	return w
}

func (actors flowActors) getA() *queries.Wallet {
	return actors.get("a")
}

func (actors flowActors) getB() *queries.Wallet {
	return actors.get("b")
}

func (actors flowActors) getC() *queries.Wallet {
	return actors.get("c")
}

//
type genesisWallet struct {
	Wallet *queries.Wallet
	Db     *db.Db
}

func (gw *genesisWallet) fillWallet(w *queries.Wallet, amount *decimal.Big) error {
	_, err := gw.Db.Exec(
		`insert into txs (
			from_wallet_id, to_wallet_id, type, amount, status_id
        ) values ($1, $2, 'internal', $3, (select id from tx_statuses where name = 'success'))`,
		gw.Wallet.ID, w.ID, &postgres.Decimal{V: amount},
	)
	return err
}

var _ = Describe("testing txs processing", func() {
	Init()
	database.Init()
	migrations.Init()

	// TODO move it to the fixtures package
	BeforeEachCProvide(func(d *db.Db) (*gorm.DB, error) {
		return gorm.Open("postgres", d.DB.DB)
	})

	BeforeEachCProvide(func() (nodes.ICoordinator, *mocks.ICoordinator) {
		c := &mocks.ICoordinator{}
		c.GetTxsSender(testCoinName).SetSupportInternalTxs(true)
		return c, c
	})

	BeforeEachCProvide(func() (isc.ITxsEventNotificator, *iscmocks.ITxsEventNotificator) {
		n := &iscmocks.ITxsEventNotificator{}
		return n, n
	})

	BeforeEachCProvide(func(
		d *gorm.DB, notificator isc.ITxsEventNotificator, coordinator nodes.ICoordinator,
	) (processing.IApi, helpers.IBalance) {
		balanceHelper := balance.New(coordinator, nil)
		p := processing.New(d, balanceHelper, notificator, coordinator)
		balanceHelper.ProcessingApi = p
		return p, balanceHelper
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
					tx, err := p.Send(
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
					tx, err := p.Send(
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
						func(coordinator *mocks.ICoordinator, s sourceWallet, r recipientWallet, p processing.IApi) {
							coordinator.GetAccountObserver(
								testCoinName,
							).SetAccountBalance(
								new(decimal.Big).SetFloat64(Bn),
							)
							coordinator.GetWalletObserver(
								testCoinName,
							).SetAddressBalance(
								s.Address, new(decimal.Big).SetFloat64(Bw),
							)

							tx, err := p.Send(
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
		BeforeEachCProvide(func(d *db.Db) flowActors {
			actors := flowActors{}
			for _, actorName := range []string{"a", "b", "c"} {
				w, err := queries.CreateWallet(d, queries.Wallet{
					Name:      strings.ToUpper(actorName),
					UserPhone: actorName,
					Address:   actorName,
					Coin:      queries.Coin{ShortName: testCoinName},
				})
				Expect(err).NotTo(HaveOccurred())
				actors[actorName] = &w
			}
			return actors
		})

		BeforeEachCProvide(func(d *db.Db) *genesisWallet {
			w, err := queries.CreateWallet(d, queries.Wallet{
				Name:      "genesis",
				UserPhone: "phone",
				Address:   "address",
				Coin:      queries.Coin{ShortName: testCoinName},
			})
			Expect(err).NotTo(HaveOccurred())
			return &genesisWallet{Wallet: &w, Db: d}
		})

		ItD(
			"should transfer 60 COINS from A to B, general balance should remain unchanged",
			func(p processing.IApi, actors flowActors, coordinator *mocks.ICoordinator, balances helpers.IBalance) {
				a := actors.getA()
				b := actors.getB()

				// fill a wallet
				coordinator.GetWalletObserver(testCoinName).SetAddressBalance(a.Address, new(decimal.Big).SetFloat64(100))
				coordinator.GetWalletObserver(testCoinName).SetAddressBalance(b.Address, new(decimal.Big))
				coordinator.GetAccountObserver(testCoinName).SetAccountBalance(new(decimal.Big).SetFloat64(100))

				// calc total balance before
				totalBalanceBefore := new(decimal.Big)

				aBal, err := balances.TotalWalletBalanceCtx(context.Background(), a)
				Expect(err).NotTo(HaveOccurred())

				bBal, err := balances.TotalWalletBalanceCtx(context.Background(), b)
				Expect(err).NotTo(HaveOccurred())

				totalBalanceBefore = totalBalanceBefore.Add(aBal, bBal)

				// send A -> B 60 coins
				_, err = p.Send(context.Background(), a, processing.NewWalletRecipient(b), new(decimal.Big).SetFloat64(60))
				Expect(err).NotTo(HaveOccurred())

				// check balances
				aBal, err = balances.TotalWalletBalanceCtx(context.Background(), a)
				Expect(err).NotTo(HaveOccurred())
				aBalVal, _ := aBal.Float64()
				Expect(aBalVal).To(BeEquivalentTo(40))

				bBal, err = balances.TotalWalletBalanceCtx(context.Background(), b)
				Expect(err).NotTo(HaveOccurred())
				bBalVal, _ := bBal.Float64()
				Expect(bBalVal).To(BeEquivalentTo(60))

				// check total balance after
				totalBalanceAfter := new(decimal.Big).Add(aBal, bBal)
				Expect(totalBalanceAfter.Cmp(totalBalanceBefore)).To(Equal(0))
			},
		)

		ItD(
			"should transfer 35 COINS from A to B and 40 COINS from A to C, general balance should remain unchanged",
			func(p processing.IApi, actors flowActors, coordinator *mocks.ICoordinator, balances helpers.IBalance) {
				a := actors.getA()
				b := actors.getB()
				c := actors.getC()

				// fill a wallet
				coordinator.GetWalletObserver(testCoinName).SetAddressBalance(a.Address, new(decimal.Big).SetFloat64(100))
				coordinator.GetWalletObserver(testCoinName).SetAddressBalance(b.Address, new(decimal.Big))
				coordinator.GetWalletObserver(testCoinName).SetAddressBalance(c.Address, new(decimal.Big))
				coordinator.GetAccountObserver(testCoinName).SetAccountBalance(new(decimal.Big).SetFloat64(100))

				// calc total balance before
				totalBalanceBefore := new(decimal.Big)

				aBal, err := balances.TotalWalletBalanceCtx(context.Background(), a)
				Expect(err).NotTo(HaveOccurred())

				bBal, err := balances.TotalWalletBalanceCtx(context.Background(), b)
				Expect(err).NotTo(HaveOccurred())

				cBal, err := balances.TotalWalletBalanceCtx(context.Background(), c)
				Expect(err).NotTo(HaveOccurred())

				totalBalanceBefore = new(decimal.Big).Add(aBal, new(decimal.Big).Add(bBal, cBal))

				// send A -> B 35 coins
				_, err = p.Send(context.Background(), a, processing.NewWalletRecipient(b), new(decimal.Big).SetFloat64(35))
				Expect(err).NotTo(HaveOccurred())

				// send A -> C 40 coins
				_, err = p.Send(context.Background(), a, processing.NewWalletRecipient(c), new(decimal.Big).SetFloat64(40))
				Expect(err).NotTo(HaveOccurred())

				// check balances
				aBal, err = balances.TotalWalletBalanceCtx(context.Background(), a)
				Expect(err).NotTo(HaveOccurred())
				aBalVal, _ := aBal.Float64()
				Expect(aBalVal).To(BeEquivalentTo(25))

				bBal, err = balances.TotalWalletBalanceCtx(context.Background(), b)
				Expect(err).NotTo(HaveOccurred())
				bBalVal, _ := bBal.Float64()
				Expect(bBalVal).To(BeEquivalentTo(35))

				cBal, err = balances.TotalWalletBalanceCtx(context.Background(), c)
				Expect(err).NotTo(HaveOccurred())
				cBalVal, _ := cBal.Float64()
				Expect(cBalVal).To(BeEquivalentTo(40))

				// check total balance after
				totalBalanceAfter := new(decimal.Big).Add(aBal, new(decimal.Big).Add(bBal, cBal))
				Expect(totalBalanceAfter.Cmp(totalBalanceBefore)).To(Equal(0))
			},
		)

		ItD(
			"should transfer 60 COINS from A to B then 35 COINS from B to A, general balance should remain unchanged",
			func(p processing.IApi, actors flowActors, coordinator *mocks.ICoordinator, balances helpers.IBalance) {
				a := actors.getA()
				b := actors.getB()

				// fill a wallet
				coordinator.GetWalletObserver(testCoinName).SetAddressBalance(a.Address, new(decimal.Big).SetFloat64(100))
				coordinator.GetWalletObserver(testCoinName).SetAddressBalance(b.Address, new(decimal.Big))
				coordinator.GetAccountObserver(testCoinName).SetAccountBalance(new(decimal.Big).SetFloat64(100))

				// calc total balance before
				totalBalanceBefore := new(decimal.Big)

				aBal, err := balances.TotalWalletBalanceCtx(context.Background(), a)
				Expect(err).NotTo(HaveOccurred())

				bBal, err := balances.TotalWalletBalanceCtx(context.Background(), b)
				Expect(err).NotTo(HaveOccurred())

				totalBalanceBefore = totalBalanceBefore.Add(aBal, bBal)

				// send A -> B 60 coins
				_, err = p.Send(context.Background(), a, processing.NewWalletRecipient(b), new(decimal.Big).SetFloat64(60))
				Expect(err).NotTo(HaveOccurred())

				// check balances
				aBal, err = balances.TotalWalletBalanceCtx(context.Background(), a)
				Expect(err).NotTo(HaveOccurred())
				aBalVal, _ := aBal.Float64()
				Expect(aBalVal).To(BeEquivalentTo(40))

				bBal, err = balances.TotalWalletBalanceCtx(context.Background(), b)
				Expect(err).NotTo(HaveOccurred())
				bBalVal, _ := bBal.Float64()
				Expect(bBalVal).To(BeEquivalentTo(60))

				// check total balance after
				totalBalanceAfterAtoBTx := new(decimal.Big).Add(aBal, bBal)
				Expect(totalBalanceAfterAtoBTx.Cmp(totalBalanceBefore)).To(Equal(0))

				// send B -> A 60 coins
				_, err = p.Send(context.Background(), b, processing.NewWalletRecipient(a), new(decimal.Big).SetFloat64(35))
				Expect(err).NotTo(HaveOccurred())

				// check balances
				aBal, err = balances.TotalWalletBalanceCtx(context.Background(), a)
				Expect(err).NotTo(HaveOccurred())
				aBalVal, _ = aBal.Float64()
				Expect(aBalVal).To(BeEquivalentTo(75))

				bBal, err = balances.TotalWalletBalanceCtx(context.Background(), b)
				Expect(err).NotTo(HaveOccurred())
				bBalVal, _ = bBal.Float64()
				Expect(bBalVal).To(BeEquivalentTo(25))

				// check total balance after
				totalBalanceAfterBtoATx := new(decimal.Big).Add(aBal, bBal)
				Expect(totalBalanceAfterBtoATx.Cmp(totalBalanceAfterAtoBTx)).To(Equal(0))
			},
		)

		ItD(
			"should transfer 35 COINS from A to C and 40 COINS from B to C, general balance should remain unchanged",
			func(p processing.IApi, actors flowActors, coordinator *mocks.ICoordinator, balances helpers.IBalance) {
				a := actors.getA()
				b := actors.getB()
				c := actors.getC()

				// fill a wallet
				coordinator.GetWalletObserver(testCoinName).SetAddressBalance(a.Address, new(decimal.Big).SetFloat64(50))
				coordinator.GetWalletObserver(testCoinName).SetAddressBalance(b.Address, new(decimal.Big).SetFloat64(50))
				coordinator.GetWalletObserver(testCoinName).SetAddressBalance(c.Address, new(decimal.Big))
				coordinator.GetAccountObserver(testCoinName).SetAccountBalance(new(decimal.Big).SetFloat64(100))

				// calc total balance before
				totalBalanceBefore := new(decimal.Big)

				aBal, err := balances.TotalWalletBalanceCtx(context.Background(), a)
				Expect(err).NotTo(HaveOccurred())

				bBal, err := balances.TotalWalletBalanceCtx(context.Background(), b)
				Expect(err).NotTo(HaveOccurred())

				cBal, err := balances.TotalWalletBalanceCtx(context.Background(), c)
				Expect(err).NotTo(HaveOccurred())

				totalBalanceBefore = new(decimal.Big).Add(aBal, new(decimal.Big).Add(bBal, cBal))

				// send A -> C 35 coins
				_, err = p.Send(context.Background(), a, processing.NewWalletRecipient(c), new(decimal.Big).SetFloat64(35))
				Expect(err).NotTo(HaveOccurred())

				// send B -> C 40 coins
				_, err = p.Send(context.Background(), b, processing.NewWalletRecipient(c), new(decimal.Big).SetFloat64(40))
				Expect(err).NotTo(HaveOccurred())

				// check balances
				aBal, err = balances.TotalWalletBalanceCtx(context.Background(), a)
				Expect(err).NotTo(HaveOccurred())
				aBalVal, _ := aBal.Float64()
				Expect(aBalVal).To(BeEquivalentTo(15))

				bBal, err = balances.TotalWalletBalanceCtx(context.Background(), b)
				Expect(err).NotTo(HaveOccurred())
				bBalVal, _ := bBal.Float64()
				Expect(bBalVal).To(BeEquivalentTo(10))

				cBal, err = balances.TotalWalletBalanceCtx(context.Background(), c)
				Expect(err).NotTo(HaveOccurred())
				cBalVal, _ := cBal.Float64()
				Expect(cBalVal).To(BeEquivalentTo(75))

				// check total balance after
				totalBalanceAfter := new(decimal.Big).Add(aBal, new(decimal.Big).Add(bBal, cBal))
				Expect(totalBalanceAfter.Cmp(totalBalanceBefore)).To(Equal(0))
			},
		)

		ItD(
			"should reject self tx",
			func(p processing.IApi, actors flowActors, coordinator *mocks.ICoordinator, balances helpers.IBalance) {
				a := actors.getA()
				coordinator.GetWalletObserver(testCoinName).SetAddressBalance(a.Address, new(decimal.Big).SetFloat64(100))
				coordinator.GetAccountObserver(testCoinName).SetAccountBalance(new(decimal.Big).SetFloat64(100))

				_, err := p.Send(context.Background(), a, processing.NewWalletRecipient(a), new(decimal.Big).SetFloat64(50))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("processing: self-tx forbidden"))
			},
		)
	})
})
