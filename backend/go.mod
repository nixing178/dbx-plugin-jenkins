module github.com/nixing178/dbx-plugin-jenkins

go 1.22

require github.com/t8y2/dbx/plugins/sdk/go/dbx-plugin-sdk v0.1.0

require golang.org/x/net v0.33.0

replace github.com/t8y2/dbx/plugins/sdk/go/dbx-plugin-sdk => ./third_party/dbx-plugin-sdk
