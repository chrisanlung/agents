//go:build !dev && !local

package payment

import "fmt"

// newDummyOrError always returns an error in non-dev/local builds.
// C-2 (SECURITY.md): The dummy adapter is excluded from production binaries
// at the build level (belt-and-suspenders with the runtime guard in NewDummy).
// Selecting PAYMENT_ADAPTER=dummy in a production build is a hard startup
// failure — not a silent fallback.
func newDummyOrError(env string) (MidtransClientIface, error) {
	return nil, fmt.Errorf(
		"CRITICAL SECURITY: PAYMENT_ADAPTER=dummy is not available in this binary "+
			"(built without dev/local tag); APP_ENV=%q. "+
			"Rebuild with -tags dev for local development, or set PAYMENT_ADAPTER=midtrans "+
			"(SECURITY.md C-2)",
		env,
	)
}
