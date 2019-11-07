package rest

import "testing"

func TestURL(t *testing.T) {
	u := NewURL("http://foobar.com")
	match(t, u, "http://foobar.com")
	u.Join("v1")
	match(t, u, "http://foobar.com/v1")
	u.Join("pets/")
	match(t, u, "http://foobar.com/v1/pets/")
	u.Join("/{id}")
	match(t, u, "http://foobar.com/v1/pets/{id}")
	u.Param("id", "lucky")
	match(t, u, "http://foobar.com/v1/pets/lucky")
	u.Param("gender", "male")
	match(t, u, "http://foobar.com/v1/pets/lucky?gender=male")
	u.Param("gender", "female")
	match(t, u, "http://foobar.com/v1/pets/lucky?gender=male&gender=female")
}

func match(t *testing.T, u *URL, match string) {
	url := u.Encode()
	if url != match {
		t.Fatalf("Mismatch: exp='%s', act='%s'", match, url)
	}
}
