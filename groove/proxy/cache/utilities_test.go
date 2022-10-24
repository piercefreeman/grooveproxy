package cache

import (
	"testing"
)

type TestSimpleObject struct {
	Value string
}

func TestEncodeDecodeObject(t *testing.T) {
	object := &TestSimpleObject{
		Value: "test",
	}

	encodedObject, err := objectToBytes(object)

	if err != nil {
		t.Fatalf("Error encoding object: %s", err)
	}

	var objectRecovered TestSimpleObject
	err = objectFromBytes(encodedObject, &objectRecovered)

	if err != nil {
		t.Fatalf("Error getting object: %s", err)
	}

	if objectRecovered.Value != object.Value {
		t.Fatalf("Recovered object does not match original (%s vs. %s)", objectRecovered.Value, object.Value)
	}
}
