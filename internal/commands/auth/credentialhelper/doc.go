package credentialhelper

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -typed -destination=./mocks_for_test.go -package=credentialhelper "golang.org/x/oauth2" "TokenSource"
