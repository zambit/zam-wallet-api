#/bin/bash

echo 'env: staging
db:
    uri: '$STAGING_DB_URI'
server:
    storage:
        uri: '$STAGING_REDIS_URI'
    auth:
        tokenstorage: jwtpersistent
    jwt:
        secret: '$STAGING_SECRET'
        method: HS256
    notificator:
        url: '$NOTIFICATIONS_URL'
    InternalAccessToken: '$ISC_ACCESS_TOKEN'

    Frontend:
        RootURL: '$FRONTEND_URL'

    Convert:
        Type: cryptocompare
        FallbackType: icex
        FallbackTimeout: 2s
isc:
    brokeruri: '$BROKER_URI'
    statsenabled: false
    statspath: /internal/stats

Logging:
    ErrorReporter:
        DSN: '$SENTRY_DSN'
    LogLevel: debug

Wallets:
    UserReporter: true
    BTC:
        NeedConfirmationsCount: 6
    cryptonodes:
        ' && [ ! -z "$BTC_HOST" ] && echo '
        btc:
            host: '$BTC_HOST'
            user: '$BTC_USER'
            pass: '$BTC_PASS'
            testnet: '$BTC_TESTNET'
        ' && [ ! -z "$BCH_HOST" ] && echo '
        bch:
            host: '$BCH_HOST'
            user: '$BCH_USER'
            pass: '$BCH_PASS'
            testnet: '$BCH_TESTNET'
        eth:
            host: '$ETH_HOST'
            testnet: '$ETH_TESTNET'
    ETH:
       EtherscanHost: '$ETHSCAN_HOST'
       EtherscanApiKey: '$ETHSCAN_KEY'
       MasterPass: '$ETH_MASTERPASS'
'