package webui

// The console speaks English, Czech and German: a demo's audience often does
// not read English fluently, so the chrome follows the browser's language on
// first visit (Accept-Language) and a header switcher after that. English
// strings are the catalog keys — an untranslated string falls back to itself.
// The chosen language also becomes the installed Moodle site's default.

import (
	"net/http"
	"time"

	"golang.org/x/text/language"
)

const langCookie = "mdl_demo_lang"

// langCodes[i] belongs to langTags[i]; index 0 is the fallback.
var (
	langCodes = []string{"en", "cs", "de"}
	langTags  = []language.Tag{language.English, language.Czech, language.German}
	langMatch = language.NewMatcher(langTags)
)

// requestLang returns the display language for this request: the cookie when
// it names a supported language, else the best Accept-Language match.
func requestLang(r *http.Request) string {
	if c, err := r.Cookie(langCookie); err == nil {
		for _, code := range langCodes {
			if c.Value == code {
				return code
			}
		}
	}
	_, idx := language.MatchStrings(langMatch, r.Header.Get("Accept-Language"))
	return langCodes[idx]
}

// handleLang sets the language cookie and bounces back. Deliberately a GET
// without auth: the login page needs the switcher too, and a display-language
// cookie is nothing worth forging.
func (s *Server) handleLang(w http.ResponseWriter, r *http.Request) {
	set := r.FormValue("set")
	for _, code := range langCodes {
		if set == code {
			http.SetCookie(w, &http.Cookie{
				Name: langCookie, Value: code, Path: "/",
				MaxAge: int((365 * 24 * time.Hour).Seconds()), SameSite: http.SameSiteLaxMode,
			})
			break
		}
	}
	back := r.Referer()
	if back == "" {
		back = "/"
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// tr is the templates' "t" function.
func tr(lang, s string) string {
	if m, ok := messages[lang]; ok {
		if t, ok := m[s]; ok {
			return t
		}
	}
	return s
}

var messages = map[string]map[string]string{
	"cs": {
		"Moodle demo console": "konzole demo Moodlu",
		"Log out":             "Odhlásit se",
		"— log in":            "— přihlášení",
		"— first-time setup":  "— první spuštění",
		"Management password": "Heslo správy",
		"Log in":              "Přihlásit se",
		"Set the management password for this container.":     "Nastavte heslo správy tohoto kontejneru.",
		"(You can also provide it at container creation with": "(Lze je také zadat při vytvoření kontejneru pomocí",
		"New password":    "Nové heslo",
		"Repeat password": "Heslo znovu",
		"Set password":    "Nastavit heslo",

		"Demo site":   "Demo stránky",
		"Recipe":      "Recept",
		"URL":         "URL",
		"Installed":   "Nainstalováno",
		"tunnel":      "tunel",
		"Reset site…": "Smazat stránky…",
		"Wipe the demo site? The database, code tree and all data are deleted.": "Opravdu smazat demo stránky? Databáze, kód i všechna data budou odstraněny.",
		"Back up data…":                 "Zálohovat data…",
		"Restore backup…":               "Obnovit zálohu…",
		"Coming soon":                   "Již brzy",
		"Stop tunnel":                   "Zastavit tunel",
		"Quick Tunnel…":                 "Quick Tunnel…",
		"Working — see progress below.": "Pracuje se — průběh je níže.",
		"No demo site installed yet.":   "Zatím nejsou nainstalovány žádné demo stránky.",
		"Install a demo site…":          "Nainstalovat demo stránky…",

		"Accounts":     "Účty",
		"User":         "Uživatel",
		"Password":     "Heslo",
		"Log in…":      "Přihlásit…",
		"Create user…": "Vytvořit uživatele…",
		"Site admin":   "Správce stránek",
		"Manager":      "Manažer",

		"Services": "Služby",
		"Service":  "Služba",
		"Status":   "Stav",

		"Progress log": "Záznam průběhu",
		"running":      "běží",
		"failed":       "selhalo",
		"done":         "hotovo",

		"Something misbehaving?":                       "Něco zlobí?",
		"The diagnostics page":                         "Diagnostická stránka",
		"has a report you can copy into a bug report.": "obsahuje výpis, který můžete vložit do hlášení chyby.",
		"— diagnostics":                                "— diagnostika",
		"← back":                                       "← zpět",

		"— install a demo site":                "— instalace demo stránek",
		"Site recipe":                          "Recept stránek",
		"Site name":                            "Název stránek",
		"Short name (shown in the navigation)": "Krátký název (zobrazí se v navigaci)",
		"Install":                              "Instalovat",
		"A strong Moodle admin password is generated automatically and shown in the Accounts section once the site is ready. Installation clones several git repositories and runs the Moodle installer — expect several minutes.": "Silné heslo správce Moodlu se vygeneruje automaticky a po dokončení instalace bude zobrazeno v sekci Účty. Instalace klonuje několik git repozitářů a spouští instalátor Moodlu — počítejte s několika minutami.",

		"Open the demo site as": "Otevřít demo stránky jako",
		"— in a new tab here, or on a phone via a single-use QR code.": "— v nové záložce zde, nebo na telefonu přes jednorázový QR kód.",
		"Log in as":         "Přihlásit jako",
		"QR code…":          "QR kód…",
		"Scan to log in as": "Naskenujte pro přihlášení jako",
		"Single use — this dialog closes once the code is claimed; open it again for the next person.": "Jen na jedno použití — po uplatnění kódu se okno zavře; pro dalšího člověka je otevřete znovu.",

		"Username":          "Uživatelské jméno",
		"First name":        "Křestní jméno",
		"Last name":         "Příjmení",
		"Global role":       "Globální role",
		"None (plain user)": "Žádná (běžný uživatel)",
		"Administrator":     "Správce",
		"Create user":       "Vytvořit uživatele",

		"Theme": "Vzhled",

		"Too many failed attempts — try again later.": "Příliš mnoho neúspěšných pokusů — zkuste to později.",
		"Wrong password.":                                                "Nesprávné heslo.",
		"Password must be at least 8 characters.":                        "Heslo musí mít alespoň 8 znaků.",
		"Passwords do not match.":                                        "Hesla se neshodují.",
		"Copy the whole block below into a bug report":                   "Celý blok níže zkopírujte do hlášení chyby",
		"It contains service states and recent log lines, no passwords.": "Obsahuje stavy služeb a poslední řádky logů, žádná hesla.",
		"the recipe's name":                                              "název receptu",
		"lowercase letters, digits, . _ -":                               "malá písmena, číslice, . _ -",

		"Mail": "Pošta",
		"Everything the demo site sends lands here — no mail ever leaves the container.": "Vše, co demo stránky odesílají, skončí zde — žádný e-mail nikdy neopustí kontejner.",
		"Open the mail catcher…": "Otevřít zachycenou poštu…",
	},
	"de": {
		"Moodle demo console": "Moodle-Demo-Konsole",
		"Log out":             "Abmelden",
		"— log in":            "— Anmeldung",
		"— first-time setup":  "— Ersteinrichtung",
		"Management password": "Verwaltungspasswort",
		"Log in":              "Anmelden",
		"Set the management password for this container.":     "Legen Sie das Verwaltungspasswort für diesen Container fest.",
		"(You can also provide it at container creation with": "(Es kann auch beim Erstellen des Containers angegeben werden mit",
		"New password":    "Neues Passwort",
		"Repeat password": "Passwort wiederholen",
		"Set password":    "Passwort festlegen",

		"Demo site":   "Demo-Website",
		"Recipe":      "Rezept",
		"URL":         "URL",
		"Installed":   "Installiert",
		"tunnel":      "Tunnel",
		"Reset site…": "Website zurücksetzen…",
		"Wipe the demo site? The database, code tree and all data are deleted.": "Demo-Website wirklich löschen? Datenbank, Code und alle Daten werden entfernt.",
		"Back up data…":                 "Daten sichern…",
		"Restore backup…":               "Sicherung wiederherstellen…",
		"Coming soon":                   "Demnächst",
		"Stop tunnel":                   "Tunnel stoppen",
		"Quick Tunnel…":                 "Quick Tunnel…",
		"Working — see progress below.": "In Arbeit — Fortschritt siehe unten.",
		"No demo site installed yet.":   "Noch keine Demo-Website installiert.",
		"Install a demo site…":          "Demo-Website installieren…",

		"Accounts":     "Konten",
		"User":         "Benutzer",
		"Password":     "Passwort",
		"Log in…":      "Anmelden…",
		"Create user…": "Benutzer anlegen…",
		"Site admin":   "Website-Administrator",
		"Manager":      "Manager",

		"Services": "Dienste",
		"Service":  "Dienst",
		"Status":   "Status",

		"Progress log": "Fortschrittsprotokoll",
		"running":      "läuft",
		"failed":       "fehlgeschlagen",
		"done":         "fertig",

		"Something misbehaving?":                       "Funktioniert etwas nicht?",
		"The diagnostics page":                         "Die Diagnoseseite",
		"has a report you can copy into a bug report.": "enthält einen Bericht zum Kopieren in eine Fehlermeldung.",
		"— diagnostics":                                "— Diagnose",
		"← back":                                       "← zurück",

		"— install a demo site":                "— Demo-Website installieren",
		"Site recipe":                          "Website-Rezept",
		"Site name":                            "Name der Website",
		"Short name (shown in the navigation)": "Kurzname (in der Navigation sichtbar)",
		"Install":                              "Installieren",
		"A strong Moodle admin password is generated automatically and shown in the Accounts section once the site is ready. Installation clones several git repositories and runs the Moodle installer — expect several minutes.": "Ein starkes Moodle-Administratorpasswort wird automatisch erzeugt und nach Abschluss der Installation im Bereich Konten angezeigt. Die Installation klont mehrere Git-Repositories und führt den Moodle-Installer aus — rechnen Sie mit einigen Minuten.",

		"Open the demo site as": "Demo-Website öffnen als",
		"— in a new tab here, or on a phone via a single-use QR code.": "— hier in einem neuen Tab oder am Telefon über einen Einmal-QR-Code.",
		"Log in as":         "Anmelden als",
		"QR code…":          "QR-Code…",
		"Scan to log in as": "Scannen zur Anmeldung als",
		"Single use — this dialog closes once the code is claimed; open it again for the next person.": "Einmalig gültig — der Dialog schließt sich, sobald der Code eingelöst ist; für die nächste Person einfach erneut öffnen.",

		"Username":          "Anmeldename",
		"First name":        "Vorname",
		"Last name":         "Nachname",
		"Global role":       "Globale Rolle",
		"None (plain user)": "Keine (normaler Benutzer)",
		"Administrator":     "Administrator",
		"Create user":       "Benutzer anlegen",

		"Theme": "Darstellung",

		"Too many failed attempts — try again later.": "Zu viele Fehlversuche — versuchen Sie es später erneut.",
		"Wrong password.":                                                "Falsches Passwort.",
		"Password must be at least 8 characters.":                        "Das Passwort muss mindestens 8 Zeichen haben.",
		"Passwords do not match.":                                        "Die Passwörter stimmen nicht überein.",
		"Copy the whole block below into a bug report":                   "Kopieren Sie den gesamten Block unten in eine Fehlermeldung",
		"It contains service states and recent log lines, no passwords.": "Er enthält Dienststatus und aktuelle Logzeilen, keine Passwörter.",
		"the recipe's name":                                              "Name des Rezepts",
		"lowercase letters, digits, . _ -":                               "Kleinbuchstaben, Ziffern, . _ -",

		"Mail": "E-Mail",
		"Everything the demo site sends lands here — no mail ever leaves the container.": "Alles, was die Demo-Website versendet, landet hier — keine E-Mail verlässt jemals den Container.",
		"Open the mail catcher…": "Postfang öffnen…",
	},
}
