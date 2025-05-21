package utils

import (
	"fmt"
	"testing"
)

func TestExtract(t *testing.T) {

	content := "结果如下，请解析：\n{\"lvl1\":{\"lvl2\":{\"lvl3\":{\"lvl4\":{\"lvl5\":[1,2,3]}}}}}\n}"
	ans, _ := ExtractJSONFromString(content)
	fmt.Println(Encode(ans))
}
