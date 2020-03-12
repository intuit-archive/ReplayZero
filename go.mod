module github.com/intuit/replay-zero

go 1.14

require (
	github.com/ProtonMail/gopenpgp/v2 v2.0.0
	github.com/aws/aws-sdk-go v1.28.14
	github.com/go-test/deep v1.0.5
	github.com/kylelemons/godebug v1.1.0
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/spf13/pflag v1.0.5
	github.com/ztrue/shutdown v0.1.1
	golang.org/x/net v0.0.0-20191126235420-ef20fe5d7933 // indirect
)

replace golang.org/x/crypto => github.com/ProtonMail/crypto v0.0.0-20191122234321-e77a1f03baa0
