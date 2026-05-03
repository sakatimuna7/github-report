package sheets

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config, tokFile string) (*http.Client, error) {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok, err = getTokenFromWeb(config)
		if err != nil {
			return nil, err
		}
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok), nil
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	// Set redirect URL to localhost so the browser can send the code back to us
	config.RedirectURL = "http://localhost:8080"
	
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	
	fmt.Printf("\n🚀 Membuka browser untuk login Google...\nJika tidak terbuka otomatis, klik link berikut:\n\n%v\n\n", authURL)
	
	// Coba buka browser otomatis (macOS/Linux/Windows)
	_ = exec.Command("open", authURL).Start() // macOS
	_ = exec.Command("xdg-open", authURL).Start() // Linux
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", authURL).Start() // Windows

	codeCh := make(chan string)
	errCh := make(chan error)
	
	srv := &http.Server{Addr: ":8080"}
	
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			fmt.Fprintf(w, "Error: Tidak ada kode otorisasi di URL.")
			errCh <- fmt.Errorf("no code found")
			return
		}
		
		fmt.Fprintf(w, "<html><body style='font-family: sans-serif; text-align: center; padding-top: 50px;'><h1>✅ Login Berhasil!</h1><p>Kredensial telah dikirim ke terminal. Silakan tutup jendela ini.</p></body></html>")
		codeCh <- code
	})
	
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	
	var authCode string
	select {
	case code := <-codeCh:
		authCode = code
	case err := <-errCh:
		srv.Shutdown(context.Background())
		return nil, fmt.Errorf("gagal menerima authorization code: %v", err)
	}
	
	// Matikan server setelah dapat kode
	srv.Shutdown(context.Background())

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %v", err)
	}
	return tok, nil
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func NewService(credFile, tokFile string) (*sheets.Service, error) {
	ctx := context.Background()
	b, err := os.ReadFile(credFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %v", err)
	}
	
	client, err := getClient(config, tokFile)
	if err != nil {
		return nil, err
	}

	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Sheets client: %v", err)
	}

	return srv, nil
}
