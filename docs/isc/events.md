# Transactions events

Events which occurs when transaction sents, transaction status changes etc

### **EVENT:** `txs.declined.{tx_id}`

Transaction has been declined due to error.

Params:

1) `coin`
    * Type: `string`
    * Description: wallet coin short name in lowercase

2) `type`
    * Type: `enum[internal, external]`
    * Description: external tx performed explicitly in blockchain, internal uses mutual settlements. 

3) `from_phone`
    * Type: `string`
    * Description: phone number of user which sends transaction

4) `from_wallet_name`
    * Type: `string`
    * Description: name of wallet used to perform transaction

5) `to_phone`
    * Type: `string`
    * Description: recipient phone number, not filled when tx is `external`

7) `to_address`
    * Type: `string`
    * Description: recipient blockhain-native wallet address, not filled when tx is `internal`

8) `amount`
    * Type: `number`
    * Description: transaction amount in coin default units

9) `error`
    * Type: string
    * Description: error due to which transaction has been declined, one of:
        * `processing: tx is exceed amount threshold`
        * `processing: insufficient funds`

### **EVENT:** `txs.processed.{tx_id}`

Transaction has been succesfully processed.

Params:

1) `coin`
    * Type: `string`
    * Description: wallet coin short name in lowercase

2) `type`
    * Type: `enum[internal, external]`
    * Description: external tx performed explicitly in blockchain, internal uses mutual settlements

3) `from_phone`
    * Type: `string`
    * Description: phone number of user which sends transaction

4) `from_wallet_name`
    * Type: `string`
    * Description: name of wallet used to perform transaction

5) `to_phone`
    * Type: `string`
    * Description: recipient phone number

7) `to_address`
    * Type: `string`
    * Description: recipient blockhain-native wallet address

8) `amount`
    * Type: `number`
    * Description: transaction amount in coin default units


### **EVENT:** `txs.awaits_recipient.{tx_id}`

Transaction awaits recipient wallet. Such events emitted only for `internal` txs.

Params:

1) `coin`
    * Type: `string`
    * Description: wallet coin short name in lowercase

3) `from_phone`
    * Type: `string`
    * Description: phone number of user which sends transaction

4) `from_wallet_name`
    * Type: `string`
    * Description: name of wallet used to perform transaction

5) `to_phone`
    * Type: `string`
    * Description: recipient phone number, not filled when tx is `external`

8) `amount`
    * Type: `number`
    * Description: transaction amount in coin default units
