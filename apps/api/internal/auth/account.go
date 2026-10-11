package auth

import (
	"net/mail"
	"strings"
	"unicode/utf8"
)

// InputError is a sign-up problem the customer can fix.
type InputError struct {
	Message string
}

func (e InputError) Error() string {
	return e.Message
}

func signupProblem(name, phone, area, email, password string) string {
	if problem := profileProblem(name, phone, area); problem != "" {
		return problem
	}
	if !validEmail(email) {
		return "Use a real email address."
	}
	if utf8.RuneCountInString(password) < 8 {
		return "Use a password of at least 8 characters."
	}
	if len(password) > 72 {
		return "Use a shorter password."
	}
	return ""
}

func profileProblem(name, phone, area string) string {
	name = strings.TrimSpace(name)
	area = strings.TrimSpace(area)
	if utf8.RuneCountInString(name) < 2 {
		return "Add the name we should call you."
	}
	if utf8.RuneCountInString(name) > 80 {
		return "Keep the name under 80 characters."
	}
	if !ugandaMobile(phone) {
		return "Use an MTN or Airtel number, like 0772 000 111."
	}
	if utf8.RuneCountInString(area) < 2 {
		return "Add your area in Kabale."
	}
	if utf8.RuneCountInString(area) > 80 {
		return "Keep the area under 80 characters."
	}
	return ""
}

func validEmail(raw string) bool {
	email := strings.ToLower(strings.TrimSpace(raw))
	if len(email) > 254 || strings.Contains(email, " ") {
		return false
	}
	parsed, err := mail.ParseAddress(email)
	return err == nil && parsed.Address == email && strings.Contains(email, ".")
}

func ugandaMobile(raw string) bool {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, raw)
	return (len(digits) == 10 && strings.HasPrefix(digits, "07")) || (len(digits) == 12 && strings.HasPrefix(digits, "2567"))
}
