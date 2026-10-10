package identity

import "testing"

func TestIsStaff(t *testing.T) {
	if IsStaff([]string{RoleCustomer}) {
		t.Fatal("customer is not staff")
	}
	if !IsStaff([]string{RoleCustomer, RoleSuperAdmin}) {
		t.Fatal("super admin is staff")
	}
	if HasAny([]string{RoleVendor}, RoleFinance) {
		t.Fatal("vendor is not finance")
	}
}
