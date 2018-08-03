alter table coins add column user_default boolean default false;
update coins set user_default = true where short_name = 'BTC';