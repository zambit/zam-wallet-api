CREATE INDEX txs_from_wallet_id_idx1 ON public.txs (from_wallet_id);
CREATE INDEX txs_to_wallet_id_idx ON public.txs (to_wallet_id);
CREATE INDEX txs_external_tx_id_idx ON public.txs_external (tx_id);