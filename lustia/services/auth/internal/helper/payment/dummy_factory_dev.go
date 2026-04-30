//go:build dev || local

package payment

// newDummyOrError constructs the DummyProvider when the build tag
// "dev" or "local" is active. C-2: the runtime env check inside NewDummy
// provides a second layer of protection in case the binary is somehow run
// with APP_ENV != dev|local despite the build tag.
func newDummyOrError(env string) (ProviderIface, error) {
	return NewDummy(env)
}
