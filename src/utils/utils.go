package utils

import (
	"encoding/json"
)

func AssertType(t interface{}, data interface{}) (map[string]interface{}, bool) {
	result, ok := data.(map[string]interface{})
	return result, ok
}

// func parseJSON(T interface{}, data interface{}) (T, bool) {

// 	var result T
// 	json.Unmarshal([]byte(T), &result)
// 	return result, ok

// }

// func ParseJSON(T interface{}, data interface{}) (interface{}, bool) {
// 	result := reflect.New(reflect.TypeOf(T)).Interface() // Create an instance of the provided type

// 	jsonData, err := json.Marshal(data)
// 	if err != nil {
// 		return nil, false // Return false if JSON marshalling fails
// 	}

// 	if err := json.Unmarshal(jsonData, &result); err != nil {
// 		return nil, false // Return false if JSON unmarshalling fails
// 	}

// 	return result, true // Return the result and true if successful
// }

func ParseJSON(data []byte, target interface{}) bool {
	err := json.Unmarshal(data, target)
	return err == nil
}

// func ParseJSONN(T interface{}, data interface{}) {
// 	// Your JSON data as a byte slice
// 	jsonData := []byte(`{"field1": "value1", "field2": 42}`)

// 	var result T

// 	if parseJSON(jsonData, &result) {
// 		// JSON data successfully unmarshalled into the 'result' variable of type YourStruct
// 		// Use 'result' here
// 		fmt.Println("Unmarshalling successful:", result)

// 	} else {
// 		// Failed to unmarshal JSON into the specified type
// 		fmt.Println("Unmarshalling failed")
// 	}
// }

// func ParseJSONNN() {
// 	// Your JSON data as a byte slice
// 	jsonData := []byte(`{"field1": "value1", "field2": 42}`)

// 	var result YourStruct

// 	if parseJSON(jsonData, &result) {
// 		// JSON data successfully unmarshalled into the 'result' variable of type YourStruct
// 		// Use 'result' here
// 		fmt.Println("Unmarshalling successful:", result)
// 	} else {
// 		// Failed to unmarshal JSON into the specified type
// 		fmt.Println("Unmarshalling failed")
// 	}
// }
