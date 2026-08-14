package unifi

import "testing"

func TestParseUsers(t *testing.T) {
	users, err := ParseUsers(mustRead(t, "testdata/rest-user.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 3 {
		t.Fatalf("%+v", users)
	}
	if users[0].ID != "64aaaaaaaaaaaaaaaaaaaa01" || users[0].MAC != "02:00:00:00:00:01" || users[0].Name != "kvm02" {
		t.Fatalf("%+v", users[0])
	}
	if users[1].IP != "203.0.113.21" {
		t.Fatalf("fixed_ip: %+v", users[1])
	}
	if users[2].Hostname != "android-xx" || users[2].Name != "" {
		t.Fatalf("%+v", users[2])
	}
}
