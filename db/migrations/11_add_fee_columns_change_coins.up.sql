update coins set user_default = true where short_name =  ANY('{ETH,BCH}'::varchar(60)[]);

alter table txs add column blockchain_fee decimal;