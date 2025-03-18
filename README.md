# dbn-duckduck-goose

**Golang Web Service Example using Databento and DuckDB**

*https://github.com/NimbleMarkets/dbn-duckduck-goose*

This `dbn-duckduck-goose` repository contains an example Golang webservice tha uses [Databento's](https://databento.com) data-as-a-service backend and embedded [DuckDB](https://duckdb.org) for storage and query.  


## Usage

```
usage: ./bin/dbn-duckduck-goose -d <dataset> -s <schema> [opts] symbol1 symbol2 ...

  -d, --dataset string          Dataset to subscribe to 
  -e, --encoding dbn.Encoding   Encoding of the output ('dbn', 'csv', 'json') (default dbn)
  -h, --help                    Show help
  -k, --key string              Databento API key (or set 'DATABENTO_API_KEY' envvar)
  -o, --out string              Output filename for DBN stream ('-' for stdout)
  -s, --schema stringArray      Schema to subscribe to (multiple allowed)
  -i, --sin dbn.SType           Input SType of the symbols. One of instrument_id, id, instr, raw_symbol, raw, smart, continuous, parent, nasdaq, cms (default raw_symbol)
  -n, --snapshot                Enable snapshot on subscription request
  -t, --start string            Start time to request as ISO 8601 format (default: now)
  -v, --verbose                 Verbose logging
```

## Building

Builds are driven by Taskfile [Taskfile](https://taskfile.dev)). Build with `task build`.

## License

Adapted from [sample code](https://github.com/NimbleMarkets/dbn-go/blob/main/cmd/dbn-go-live/main.go) in  [NimbleMarkets `dbn-go` library](https://github.com/NimbleMarkets/dbn-go).

Released under the [MIT License](https://en.wikipedia.org/wiki/MIT_License), see [LICENSE.txt](./LICENSE.txt).

Copyright (c) 2025 [Neomantra Corp](https://www.neomantra.com).   

----
Made with :heart: and :fire: by the team behind [Nimble.Markets](https://nimble.markets).
