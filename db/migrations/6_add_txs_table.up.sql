create table tx_statuses (
  id serial primary key,
  name varchar(32) not null
);

insert into tx_statuses (name) values ('waiting'), ('decline'), ('pending'), ('cancel'), ('success');

create type tx_type as enum ('internal', 'external');
create table txs (
  id bigserial primary key,

  from_wallet_id integer references wallets(id) not null,
  to_wallet_id   integer references wallets(id) null,
  to_address     varchar(64) null,
  amount         decimal not null,
  type           tx_type not null,
  status_id      int references tx_statuses(id) not null,

  created_at timestamp without time zone default (now() at time zone 'UTC'),
  updated_at timestamp without time zone null
);

create index txs_from_wallet_id_idx on txs (
  from_wallet_id asc,
  to_wallet_id asc nulls last,
  to_address asc nulls last,
  status_id asc
);