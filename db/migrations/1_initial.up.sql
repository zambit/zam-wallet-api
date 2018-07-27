create table coins (
  id         serial primary key,
  name       varchar(20) not null,
  short_name varchar(6) not null,
  enabled    boolean default true
);

create table wallets (
  id         serial primary key,
  user_id    bigint not null,
  coin_id    integer references coins(id) not null
  name       varchar(64) not null,
  address    varchar(64) not null,
  created_at time without time zone default now() at time zone 'UTC'
);

insert into coins (name, short_name, enabled) values
  ('BTC', 'Bitcoin', true),
  ('BCH', 'Bitcoin cash', false),
  ('ETH', 'Ethereum', false),
  ('Zam', 'Zam', false);