alter table wallets add constraint wallets_unique_user_coin_pair_cst unique (user_id, coin_id);