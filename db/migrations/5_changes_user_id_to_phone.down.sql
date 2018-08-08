alter table wallets drop constraint wallets_unique_user_coin_pair_cst;
alter table wallets add constraint wallets_unique_user_coin_pair_cst unique (user_id, coin_id);
alter table wallets add constraint wallets_user_id_not_null check(user_id is not null) not valid;