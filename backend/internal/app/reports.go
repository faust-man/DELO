package app

func init() {
	resources["finance_summary"] = Resource{"id,currency,received,refunded,charged,allocated", []string{"owner", "cashier"}, nil}
}
