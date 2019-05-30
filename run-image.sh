#!/bin/bash

# Run the insight-bch Docker image
docker container run --name cashshuffle -d -p 1337:1337 -p 1338:1338 -p 8080:8080 --rm cashshuffle-server
