ALTER TABLE app_agent_file_transfers
    DROP CONSTRAINT app_agent_file_transfers_expected_size_check,
    ADD CONSTRAINT app_agent_file_transfers_expected_size_check CHECK (expected_size BETWEEN 0 AND 1073741824);
