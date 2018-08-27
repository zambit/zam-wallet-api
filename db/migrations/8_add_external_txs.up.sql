create table txs_external (
  id bigserial primary key,
  tx_id bigint references txs(id),
  hash varchar(512) not null,
  recipient varchar(256) not null
);