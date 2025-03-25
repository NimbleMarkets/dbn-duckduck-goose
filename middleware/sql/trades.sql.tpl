-- Create trades table
CREATE TABLE IF NOT EXISTS {{.TableName}} (
	date date NOT NULL,
	timestamp integer NOT NULL,
	nanos integer NOT NULL,
	publisher integer NOT NULL,
	ticker varchar(12) NOT NULL,
	price decimal(19,3) NOT NULL,
	shares integer NOT NULL
);

-- Create indices
CREATE UNIQUE INDEX IF NOT EXISTS {{.TableName}}_date_ticker_timestamp_idx ON {{.TableName}} (date, ticker, timestamp);
CREATE UNIQUE INDEX IF NOT EXISTS {{.TableName}}_ticker_date_timestamp_idx ON {{.TableName}} (ticker, date, timestamp);
CREATE UNIQUE INDEX IF NOT EXISTS {{.TableName}}_timestamp_ticker_idx ON {{.TableName}} (timestamp, ticker);
CREATE UNIQUE INDEX IF NOT EXISTS {{.TableName}}_ticker_timestamp_idx ON {{.TableName}} (ticker, timestamp);
