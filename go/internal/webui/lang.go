package webui

// The console speaks English, Czech and German: a demo's audience often does
// not read English fluently, so the chrome follows the browser's language on
// first visit (Accept-Language) and a header switcher after that. English
// strings are the catalog keys — an untranslated string falls back to itself.
// The chosen language also becomes the installed Moodle site's default.

import (
	"net/http"
	"strings"
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
// with no CSRF token: a display-language cookie is nothing worth forging.
func (s *Server) handleLang(w http.ResponseWriter, r *http.Request) {
	set := r.FormValue("set")
	for _, code := range langCodes {
		if set == code {
			http.SetCookie(w, &http.Cookie{
				Name: langCookie, Value: code, Path: "/",
				MaxAge: int((365 * 24 * time.Hour).Seconds()), SameSite: http.SameSiteStrictMode,
			})
			break
		}
	}
	// The console sends no Referer (Referrer-Policy: no-referrer in auth.go), so
	// the page to return to is passed explicitly. Accept only a local absolute
	// path — never a protocol-relative "//host" that would be an open redirect.
	back := r.FormValue("to")
	if !strings.HasPrefix(back, "/") || strings.HasPrefix(back, "//") {
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

		"Demo site":  "Demo stránky",
		"Recipe":     "Recept",
		"URL":        "URL",
		"Installed":  "Nainstalováno",
		"tunnel":     "tunel",
		"Reset site": "Smazat stránky",
		"Wipe the demo site? The database, code tree and all data are deleted.": "Opravdu smazat demo stránky? Databáze, kód i všechna data budou odstraněny.",
		"Back up data":                  "Zálohovat data",
		"Stop tunnel":                   "Zastavit tunel",
		"Quick Tunnel":                  "Quick Tunnel",
		"Working — see progress below.": "Pracuje se – průběh je níže.",
		"No demo site installed yet.":   "Zatím nejsou nainstalovány žádné demo stránky.",

		"Accounts":   "Účty",
		"User":       "Uživatel",
		"Password":   "Heslo",
		"Site admin": "Správce stránek",
		"Manager":    "Manažer",

		"Services": "Služby",
		"Service":  "Služba",
		"Status":   "Stav",

		"Site log":   "Záznam stránek",
		"installing": "instaluje se",
		"resetting":  "maže se",
		"backing up": "zálohuje se",
		"restoring":  "obnovuje se",
		"failed":     "selhalo",

		"Something misbehaving?":                       "Něco zlobí?",
		"The diagnostics page":                         "Diagnostická stránka",
		"has a report you can copy into a bug report.": "obsahuje výpis, který můžete vložit do hlášení chyby.",
		"— diagnostics":                                "– diagnostika",
		"← back":                                       "← zpět",

		"Site recipe": "Recept stránek",
		"Install":     "Instalovat",
		"A strong Moodle admin password is generated automatically and shown in the Accounts section once the site is ready. Installation clones several git repositories and runs the Moodle installer — expect several minutes.": "Silné heslo správce Moodlu se vygeneruje automaticky a po dokončení instalace bude zobrazeno v sekci Účty. Instalace klonuje několik git repozitářů a spouští instalátor Moodlu – počítejte s několika minutami.",

		"Open the demo site as": "Otevřít demo stránky jako",
		"— in a new tab here, or on a phone via a single-use QR code.": "– v nové záložce zde, nebo na telefonu přes jednorázový QR kód.",
		"in a new tab here.": "v nové záložce zde.",
		"Log in as":          "Přihlásit jako",
		"QR code":            "QR kód",
		"Scan to log in as":  "Naskenujte pro přihlášení jako",
		"Single use — this dialog closes once the code is claimed; open it again for the next person.": "Jen na jedno použití – po uplatnění kódu se okno zavře; pro dalšího člověka je otevřete znovu.",

		"Username":          "Uživatelské jméno",
		"First name":        "Křestní jméno",
		"Last name":         "Příjmení",
		"Global role":       "Globální role",
		"None (plain user)": "Žádná (běžný uživatel)",
		"Administrator":     "Správce",
		"Create user":       "Vytvořit uživatele",

		"Theme": "Vzhled",

		"Copy the whole block below into a bug report":                   "Celý blok níže zkopírujte do hlášení chyby",
		"It contains service states and recent log lines, no passwords.": "Obsahuje stavy služeb a poslední řádky logů, žádná hesla.",
		"lowercase letters, digits, . _ -":                               "malá písmena, číslice, . _ -",

		"Tools":             "Nástroje",
		"Redirected emails": "Přesměrované e-maily",
		"Plugins":           "Pluginy",
		"Public":            "Veřejné",
		"Off":               "Vypnuto",
		"Share the demo site on a public trycloudflare.com URL, so the audience can open it on their own devices.": "Sdílejte demo stránky na veřejné adrese trycloudflare.com, aby si je posluchači mohli otevřít na vlastních zařízeních.",
		"The extra plugins this site carries on top of core Moodle.":                                               "Doplňkové pluginy, které tyto stránky mají navíc oproti základnímu Moodlu.",
		"Every mail the site sends is caught here — nothing ever leaves the container.":                            "Každý e-mail, který stránky odešlou, se zachytí zde – nic nikdy neopustí kontejner.",
		"Save the site to a portable file, or restore one — even into a newer Moodle.":                             "Uložte stránky do přenosného souboru nebo některé obnovte – i do novější verze Moodlu.",
		"— plugins":          "– pluginy",
		"Additional plugins": "Doplňkové pluginy",
		"The plugins this demo site carries on top of the ones Moodle ships with.": "Pluginy, které tyto demo stránky mají navíc oproti těm, které jsou součástí Moodlu.",
		"Plugin":    "Plugin",
		"Component": "Komponenta",
		"Version":   "Verze",
		"This site carries no additional plugins — only the ones Moodle ships with.": "Tyto stránky nemají žádné doplňkové pluginy – jen ty, které jsou součástí Moodlu.",

		"No demo site installed yet — install a fresh site from a recipe, or restore a backup.": "Zatím nejsou nainstalovány žádné demo stránky – nainstalujte nové z receptu, nebo obnovte zálohu.",
		"Install a fresh site": "Instalace nových stránek",
		"Restore a backup":     "Obnovení zálohy",
		"Install this recipe now? Expect several minutes.": "Nainstalovat tento recept? Počítejte s několika minutami.",
		"Restore this backup now? Expect a few minutes.":   "Obnovit tuto zálohu? Počítejte s několika minutami.",
		"older versions":  "starší verze",
		"Release":         "Vydání",
		"Development":     "Vývojová verze",
		"Full suite":      "Kompletní sada",
		"On stock Moodle": "Na čistém Moodlu",
		"Unmodified Moodle, latest point releases — plain core, no plugins.":    "Neupravený Moodle, nejnovější dílčí vydání – čisté jádro, žádné pluginy.",
		"Unreleased Moodle from the active branches — a look at what's coming.": "Dosud nevydaný Moodle z aktivních větví – náhled na to, co přijde.",
		"Patched Moodle core with multi-tenancy and every MuTMS plugin.":        "Upravené jádro Moodlu s více nájemci (multi-tenancy) a všemi pluginy MuTMS.",
		"All MuTMS plugins on unmodified Moodle core — no multi-tenancy.":       "Všechny pluginy MuTMS na neupraveném jádře Moodlu – bez více nájemců.",
		"The full MuTMS suite tracking the latest Moodle branches.":             "Kompletní sada MuTMS sledující nejnovější větve Moodlu.",
		"Site name":                            "Název stránek",
		"Short name (shown in the navigation)": "Krátký název (zobrazí se v navigaci)",
		"the recipe's name":                    "název receptu",

		"— backups": "– zálohy",
		"Back up":   "Zálohování",
		"A backup packs the database, the uploaded files and the site's exact code recipe into one portable .mdb file.": "Záloha zabalí databázi, nahrané soubory a přesný recept kódu stránek do jednoho přenosného souboru .mdb.",
		"Create a backup now? The site is briefly unavailable while it is copied.":                                      "Vytvořit zálohu nyní? Stránky budou během kopírování krátce nedostupné.",
		"Upload": "Nahrát",
		"Upload a .mdb file created here or by another demo container.": "Nahrajte soubor .mdb vytvořený zde nebo jiným demo kontejnerem.",
		"The site's code is rebuilt from the selected recipe, then the backup's data is loaded and upgraded to match. The bundled recipe restores the exact original site.": "Kód stránek se znovu sestaví podle vybraného receptu, poté se nahrají data zálohy a povýší se na odpovídající verzi. Přibalený recept obnoví přesně původní stránky.",
		"Replace the current demo site with this backup? The existing database, code tree and data are deleted.":                                                            "Nahradit současné demo stránky touto zálohou? Stávající databáze, kód i data budou odstraněny.",
		"Bundled recipe (exact original)": "Přibalený recept (přesný originál)",
		"Backups":                         "Zálohy",
		"File":                            "Soubor",
		"Created":                         "Vytvořeno",
		"Size":                            "Velikost",
		"not a recognized backup archive": "nerozpoznaný formát zálohy",
		"Download":                        "Stáhnout",
		"Restore":                         "Obnovit",
		"Delete this backup file?":        "Smazat tento soubor zálohy?",
		"Delete":                          "Smazat",
		"No backups yet.":                 "Zatím žádné zálohy.",
		"Backups survive a site reset but live inside this container — deleting the container deletes them. Download the ones you want to keep.": "Zálohy přežijí smazání stránek, ale žijí uvnitř tohoto kontejneru – smazáním kontejneru zaniknou. Ty, které si chcete ponechat, si stáhněte.",
	},
	"de": {
		"Moodle demo console": "Moodle-Demo-Konsole",

		"Demo site":  "Demo-Website",
		"Recipe":     "Rezept",
		"URL":        "URL",
		"Installed":  "Installiert",
		"tunnel":     "Tunnel",
		"Reset site": "Website zurücksetzen",
		"Wipe the demo site? The database, code tree and all data are deleted.": "Demo-Website wirklich löschen? Datenbank, Code und alle Daten werden entfernt.",
		"Back up data":                  "Daten sichern",
		"Stop tunnel":                   "Tunnel stoppen",
		"Quick Tunnel":                  "Quick Tunnel",
		"Working — see progress below.": "In Arbeit – Fortschritt siehe unten.",
		"No demo site installed yet.":   "Noch keine Demo-Website installiert.",

		"Accounts":   "Konten",
		"User":       "Benutzer",
		"Password":   "Passwort",
		"Site admin": "Website-Administrator",
		"Manager":    "Manager",

		"Services": "Dienste",
		"Service":  "Dienst",
		"Status":   "Status",

		"Site log":   "Website-Protokoll",
		"installing": "wird installiert",
		"resetting":  "wird zurückgesetzt",
		"backing up": "wird gesichert",
		"restoring":  "wird wiederhergestellt",
		"failed":     "fehlgeschlagen",

		"Something misbehaving?":                       "Funktioniert etwas nicht?",
		"The diagnostics page":                         "Die Diagnoseseite",
		"has a report you can copy into a bug report.": "enthält einen Bericht zum Kopieren in eine Fehlermeldung.",
		"— diagnostics":                                "– Diagnose",
		"← back":                                       "← zurück",

		"Site recipe": "Website-Rezept",
		"Install":     "Installieren",
		"A strong Moodle admin password is generated automatically and shown in the Accounts section once the site is ready. Installation clones several git repositories and runs the Moodle installer — expect several minutes.": "Ein starkes Moodle-Administratorpasswort wird automatisch erzeugt und nach Abschluss der Installation im Bereich Konten angezeigt. Die Installation klont mehrere Git-Repositories und führt den Moodle-Installer aus – rechnen Sie mit einigen Minuten.",

		"Open the demo site as": "Demo-Website öffnen als",
		"— in a new tab here, or on a phone via a single-use QR code.": "– hier in einem neuen Tab oder am Telefon über einen Einmal-QR-Code.",
		"in a new tab here.": "hier in einem neuen Tab.",
		"Log in as":          "Anmelden als",
		"QR code":            "QR-Code",
		"Scan to log in as":  "Scannen zur Anmeldung als",
		"Single use — this dialog closes once the code is claimed; open it again for the next person.": "Einmalig gültig – der Dialog schließt sich, sobald der Code eingelöst ist; für die nächste Person einfach erneut öffnen.",

		"Username":          "Anmeldename",
		"First name":        "Vorname",
		"Last name":         "Nachname",
		"Global role":       "Globale Rolle",
		"None (plain user)": "Keine (normaler Benutzer)",
		"Administrator":     "Administrator",
		"Create user":       "Benutzer anlegen",

		"Theme": "Darstellung",

		"Copy the whole block below into a bug report":                   "Kopieren Sie den gesamten Block unten in eine Fehlermeldung",
		"It contains service states and recent log lines, no passwords.": "Er enthält Dienststatus und aktuelle Logzeilen, keine Passwörter.",
		"lowercase letters, digits, . _ -":                               "Kleinbuchstaben, Ziffern, . _ -",

		"Tools":             "Werkzeuge",
		"Redirected emails": "Umgeleitete E-Mails",
		"Plugins":           "Plugins",
		"Public":            "Öffentlich",
		"Off":               "Aus",
		"Share the demo site on a public trycloudflare.com URL, so the audience can open it on their own devices.": "Teilen Sie die Demo-Website unter einer öffentlichen trycloudflare.com-Adresse, damit das Publikum sie auf eigenen Geräten öffnen kann.",
		"The extra plugins this site carries on top of core Moodle.":                                               "Die zusätzlichen Plugins dieser Website über das Moodle-Kernsystem hinaus.",
		"Every mail the site sends is caught here — nothing ever leaves the container.":                            "Jede E-Mail, die die Website sendet, wird hier abgefangen – nichts verlässt jemals den Container.",
		"Save the site to a portable file, or restore one — even into a newer Moodle.":                             "Sichern Sie die Website in eine portable Datei oder stellen Sie eine wieder her – auch in einer neueren Moodle-Version.",
		"— plugins":          "– Plugins",
		"Additional plugins": "Zusätzliche Plugins",
		"The plugins this demo site carries on top of the ones Moodle ships with.": "Die Plugins, die diese Demo-Website zusätzlich zu den mit Moodle gelieferten enthält.",
		"Plugin":    "Plugin",
		"Component": "Komponente",
		"Version":   "Version",
		"This site carries no additional plugins — only the ones Moodle ships with.": "Diese Website hat keine zusätzlichen Plugins – nur die mit Moodle gelieferten.",

		"No demo site installed yet — install a fresh site from a recipe, or restore a backup.": "Noch keine Demo-Website installiert – installieren Sie eine neue aus einem Rezept oder stellen Sie eine Sicherung wieder her.",
		"Install a fresh site": "Neue Website installieren",
		"Restore a backup":     "Sicherung wiederherstellen",
		"Install this recipe now? Expect several minutes.": "Dieses Rezept jetzt installieren? Rechnen Sie mit einigen Minuten.",
		"older versions":  "ältere Versionen",
		"Release":         "Release",
		"Development":     "Entwicklung",
		"Full suite":      "Komplettpaket",
		"On stock Moodle": "Auf reinem Moodle",
		"Unmodified Moodle, latest point releases — plain core, no plugins.":    "Unverändertes Moodle, neueste Point-Releases – reiner Kern, keine Plugins.",
		"Unreleased Moodle from the active branches — a look at what's coming.": "Noch unveröffentlichtes Moodle aus den aktiven Branches – ein Blick auf das Kommende.",
		"Patched Moodle core with multi-tenancy and every MuTMS plugin.":        "Gepatchter Moodle-Kern mit Mandantenfähigkeit und allen MuTMS-Plugins.",
		"All MuTMS plugins on unmodified Moodle core — no multi-tenancy.":       "Alle MuTMS-Plugins auf unverändertem Moodle-Kern – ohne Mandantenfähigkeit.",
		"The full MuTMS suite tracking the latest Moodle branches.":             "Das komplette MuTMS-Paket, das den neuesten Moodle-Branches folgt.",
		"Site name":                                      "Name der Website",
		"Short name (shown in the navigation)":           "Kurzname (in der Navigation sichtbar)",
		"the recipe's name":                              "Name des Rezepts",
		"Restore this backup now? Expect a few minutes.": "Diese Sicherung jetzt wiederherstellen? Rechnen Sie mit einigen Minuten.",

		"— backups": "– Sicherungen",
		"Back up":   "Sichern",
		"A backup packs the database, the uploaded files and the site's exact code recipe into one portable .mdb file.": "Eine Sicherung packt die Datenbank, die hochgeladenen Dateien und das genaue Code-Rezept der Website in eine portable .mdb-Datei.",
		"Create a backup now? The site is briefly unavailable while it is copied.":                                      "Jetzt eine Sicherung erstellen? Die Website ist während des Kopierens kurz nicht erreichbar.",
		"Upload": "Hochladen",
		"Upload a .mdb file created here or by another demo container.": "Laden Sie eine .mdb-Datei hoch, die hier oder von einem anderen Demo-Container erstellt wurde.",
		"The site's code is rebuilt from the selected recipe, then the backup's data is loaded and upgraded to match. The bundled recipe restores the exact original site.": "Der Code der Website wird aus dem gewählten Rezept neu aufgebaut, dann werden die Daten der Sicherung geladen und entsprechend aktualisiert. Das mitgelieferte Rezept stellt exakt die ursprüngliche Website wieder her.",
		"Replace the current demo site with this backup? The existing database, code tree and data are deleted.":                                                            "Die aktuelle Demo-Website durch diese Sicherung ersetzen? Bestehende Datenbank, Code und Daten werden entfernt.",
		"Bundled recipe (exact original)": "Mitgeliefertes Rezept (exaktes Original)",
		"Backups":                         "Sicherungen",
		"File":                            "Datei",
		"Created":                         "Erstellt",
		"Size":                            "Größe",
		"not a recognized backup archive": "kein erkennbares Sicherungsarchiv",
		"Download":                        "Herunterladen",
		"Restore":                         "Wiederherstellen",
		"Delete this backup file?":        "Diese Sicherungsdatei löschen?",
		"Delete":                          "Löschen",
		"No backups yet.":                 "Noch keine Sicherungen.",
		"Backups survive a site reset but live inside this container — deleting the container deletes them. Download the ones you want to keep.": "Sicherungen überstehen das Zurücksetzen der Website, liegen aber in diesem Container – mit dem Container werden sie gelöscht. Laden Sie herunter, was Sie behalten möchten.",
	},
}
