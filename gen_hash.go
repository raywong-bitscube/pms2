package main
import ("golang.org/x/crypto/bcrypt"; "fmt")
func main() {
	h, _ := bcrypt.GenerateFromPassword([]byte("Admin@123"), 12)
	fmt.Println(string(h))
}
