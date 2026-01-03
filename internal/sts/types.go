package sts

type stsServiceImplementation struct {
	sts internalSts
}

type GetCallerIdentityResponse struct {
	Arn     string `json:"Arn"`
	Account string `json:"Account"`
	UserId  string `json:"UserId"`
}

type stsImplementation struct {
}
