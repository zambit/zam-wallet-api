package simple_test

import (
	"testing"

	"git.zam.io/wallet-backend/wallet-api/internal/services/isc"
	"git.zam.io/wallet-backend/wallet-api/internal/services/isc/simple"
	"github.com/ericlagergren/decimal"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"git.zam.io/wallet-backend/web-api/pkg/services/notifications/mocks"
)

func TestSimpleTxsNotificator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Simple Txs Notificator Suite")
}

const (
	appUrl    = "http://localhost/"
	fromPhone = "+79991112233"
	toPhone   = "+79991112255"
	coinName  = "ETH"
)

var (
	amount = new(decimal.Big).SetFloat64(25)
)

var _ = Describe("testing txs events notificator", func() {
	transport := &mocks.ITransport{}
	notificator := simple.New(transport, appUrl)

	Describe("testing params validation", func() {
		It("should fail due to missing dst phone", func() {
			err := notificator.AwaitRecipient(isc.TxEventPayload{FromPhone: fromPhone, Coin: coinName, Amount: amount})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("simple txs event notificator: empty ToPhone field"))
		})

		It("should fail due to missing coin name", func() {
			err := notificator.AwaitRecipient(isc.TxEventPayload{ToPhone: toPhone, FromPhone: fromPhone, Amount: amount})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("simple txs event notificator: empty Coin field"))
		})

		It("should fail due to missing form phone", func() {
			err := notificator.AwaitRecipient(isc.TxEventPayload{ToPhone: toPhone, Coin: coinName, Amount: amount})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("simple txs event notificator: empty FromPhone field"))
		})

		It("should fail due to missing dst phone", func() {
			err := notificator.AwaitRecipient(isc.TxEventPayload{ToPhone: toPhone, FromPhone: fromPhone, Coin: coinName})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("simple txs event notificator: empty Amount field"))
		})

		It("should send valid message", func() {
			transport.On(
				"Send",
				toPhone,
				`Hi from Zamzam Bank! You got 25 ETH from +79991112233, in order to receive them, go through the verification of your phone at http://localhost/`,
			).Return(nil)

			err := notificator.AwaitRecipient(isc.TxEventPayload{
				ToPhone: toPhone, FromPhone: fromPhone, Coin: coinName, Amount: amount,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(transport.Calls)).To(Equal(1))
		})
	})
})

const pattern = `Hi from Zamzam Bank! You got %<amount>s %<coin>s from %<phone_number>s, in order to receive them, go through the verification of your phone at %<app_url>s`