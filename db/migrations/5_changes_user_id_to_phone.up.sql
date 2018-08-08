alter table wallets drop constraint wallets_unique_user_coin_pair_cst;
alter table wallets alter column user_id drop not null;
alter table wallets add column user_phone varchar(16);
alter table wallets add constraint wallets_unique_user_coin_pair_cst unique (user_phone, coin_id);