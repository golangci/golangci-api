package paddle

type requestAuth struct {
	VendorID       int    `schema:"vendor_id"`
	VendorAuthCode string `schema:"vendor_auth_code"`
}
