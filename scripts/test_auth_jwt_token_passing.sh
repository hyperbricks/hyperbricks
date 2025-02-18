#!/bin/zsh

curl -X GET http://localhost:8080/tokentest \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJyb2xlIjoicm9iYmVydCIsInN1YiI6InJvYmJlcnQifQ.Eh0BgeSP49hFSSR0T_F-hBuKYairaq2bVF2mwACecc4" \
  -H "Content-Type: application/json"
