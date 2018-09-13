update coins set user_default = false where short_name = ANY('{ETH,BCH}'::varchar(60)[]);

alter table txs drop column blockchain_fee;