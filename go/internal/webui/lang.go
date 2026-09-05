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
		"Moodle/MuTMS demo console": "konzole demo Moodle/MuTMS",

		"Demo site":  "Demo stránky",
		"Recipe":     "Recept",
		"URL":        "URL",
		"Installed":  "Nainstalováno",
		"tunnel":     "tunel",
		"Reset site": "Smazat stránky",
		"Cancel":     "Zrušit",
		"Confirm":    "Potvrdit",
		"Wipe the demo site? The database, code tree and all data are deleted.": "Opravdu smazat demo stránky? Databáze, kód i všechna data budou odstraněny.",
		"Back up data": "Zálohovat data",
		"Backup name":  "Název zálohy",
		"The site is briefly unavailable while it is copied.": "Stránky jsou během kopírování krátce nedostupné.",
		"Quick Tunnel":                "Quick Tunnel",
		"No demo site installed yet.": "Zatím nejsou nainstalovány žádné demo stránky.",

		"Accounts":   "Účty",
		"User":       "Uživatel",
		"Password":   "Heslo",
		"Site admin": "Správce stránek",
		"Manager":    "Manažer",

		"Service": "Služba",
		"Status":  "Stav",

		"Site log":   "Záznam stránek",
		"installing": "instaluje se",
		"resetting":  "maže se",
		"backing up": "zálohuje se",
		"restoring":  "obnovuje se",
		"failed":     "selhalo",

		"Diagnostics": "Diagnostika",
		"← back":      "← zpět",

		"Site recipe":           "Recept stránek",
		"Keep current codebase": "Ponechat stávající kód",
		"The backup's data is loaded and upgraded to match the code it runs on.": "Data ze zálohy se načtou a upgradují podle kódu, na kterém běží.",
		"Install": "Instalovat",

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

		"Settings":          "Nastavení",
		"Catalogues":        "Katalogy",
		"Update catalogues": "Aktualizovat katalogy",
		"Pull the latest plugins and site recipes from git — new versions appear in the installer.": "Stáhne nejnovější pluginy a recepty z gitu – nové verze se objeví v instalátoru.",

		"When you report a problem, copy the whole block below into your issue at":                                     "Když hlásíte problém, zkopírujte celý blok níže do hlášení na",
		"It contains service states and recent log lines, no passwords.":                                               "Obsahuje stavy služeb a poslední řádky logů, žádná hesla.",
		"The exact code tree — plugins and their git sources. Share it only if your problem is about the code itself.": "Přesný strom kódu – pluginy a jejich zdroje v gitu. Sdílejte jej jen tehdy, když se problém týká samotného kódu.",
		"Service problems":                 "Problémy se službami",
		"These services are not running:":  "Tyto služby neběží:",
		"lowercase letters, digits, . _ -": "malá písmena, číslice, . _ -",

		"Tools":             "Nástroje",
		"Redirected emails": "Přesměrované e-maily",
		"Installed plugins": "Nainstalované pluginy",
		"Recommendations":   "Doporučení",
		"Projects and companies the mdl-demo maintainer recommends.": "Projekty a firmy, které správce mdl-demo doporučuje.",
		"Refresh the code catalogues, or grab a diagnostics report.": "Aktualizujte katalogy kódu nebo získejte diagnostický výpis.",
		"Public":   "Veřejné",
		"starting": "spouští se",
		"Off":      "Vypnuto",
		"Share the demo site on a public trycloudflare.com URL, so the audience can open it on their own devices.": "Sdílejte demo stránky na veřejné adrese trycloudflare.com, aby si je posluchači mohli otevřít na vlastních zařízeních.",
		"The extra plugins this site carries on top of core Moodle.":                                               "Doplňkové pluginy, které tyto stránky mají navíc oproti základnímu Moodlu.",
		"Every mail the site sends is caught here — nothing ever leaves the container.":                            "Každý e-mail, který stránky odešlou, se zachytí zde – nic nikdy neopustí kontejner.",
		"Save the site to a portable file, or restore one — even into a newer Moodle.":                             "Uložte stránky do přenosného souboru nebo některé obnovte – i do novější verze Moodlu.",
		"The plugins this demo site carries on top of the ones Moodle ships with.":                                 "Pluginy, které tyto demo stránky mají navíc oproti těm, které jsou součástí Moodlu.",
		"Component":                      "Komponenta",
		"Type":                           "Typ",
		"Location":                       "Umístění",
		"Source":                         "Zdroj",
		"Select one for details.":        "Vyberte plugin pro podrobnosti.",
		"Add a plugin":                   "Přidat plugin",
		"This site runs Moodle":          "Tyto stránky používají Moodle",
		"pick a matching version below.": "níže vyberte odpovídající verzi.",
		"Show versions":                  "Zobrazit verze",
		"Install plugin":                 "Nainstalovat plugin",
		"Back up first — a restore point in case of breakage": "Nejprve zálohovat — bod obnovení pro případ potíží",
		"Installing from": "Instaluje se z",
		"Installing":      "Instaluje se",
		"Branches":        "Větve",
		"Tags":            "Tagy",
		"adding plugin":   "přidává se plugin",
		"Adding a plugin runs its code on this throwaway demo. If things break, reset the site; if it gets compromised, delete the container entirely.": "Přidání pluginu spustí jeho kód na této dočasné ukázce. Když se něco rozbije, smažte stránky; při kompromitaci celý kontejner smažte.",
		"Version":                              "Verze",
		"No additional plugins installed yet.": "Zatím nejsou nainstalovány žádné doplňkové pluginy.",

		"Pick a version to install a demo": "Vyberte verzi dema k instalaci",
		"older versions":                   "starší verze",
		"Release":                          "Vydání",
		"Development":                      "Vývojová verze",
		"Full suite":                       "Kompletní sada",
		"On plain Moodle":                  "Obyčejný Moodle s pluginy",
		"Plain Moodle — core only, no plugins.":                                    "Obyčejný Moodle – jen jádro, žádné pluginy.",
		"Plain Moodle, still in development — a look at what's coming.":            "Obyčejný Moodle ve vývoji – náhled na to, co přijde.",
		"Patched Moodle core with multi-tenancy and every MuTMS plugin.":           "Upravené jádro Moodlu s více nájemci (multi-tenancy) a všemi pluginy MuTMS.",
		"All MuTMS plugins on plain Moodle core — no multi-tenancy.":               "Všechny pluginy MuTMS na obyčejném jádře Moodlu – bez více nájemců.",
		"The full MuTMS suite in active development, on the latest stable Moodle.": "Kompletní sada MuTMS ve vývoji, na nejnovějším stabilním Moodlu.",
		"IOMAD, a multi-tenant Moodle distribution — its current stable branches.": "IOMAD, distribuce Moodlu s více nájemci – jeho aktuální stabilní větve.",
		"Site name": "Název stránek",

		"Back up": "Zálohování",
		"A backup packs the database, the uploaded files and the site's exact code recipe into one portable .mdb file.": "Záloha zabalí databázi, nahrané soubory a přesný recept kódu stránek do jednoho přenosného souboru .mdb.",
		"Upload": "Nahrát",
		"Upload a .mdb file created here or by another demo container.": "Nahrajte soubor .mdb vytvořený zde nebo jiným demo kontejnerem.",
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

		"CAMP": "CAMP",
		"Browse the community catalogue and test plugins in this demo site.":                       "Procházejte komunitní katalog a testujte pluginy na těchto demo stránkách.",
		"CAMP — the Community Archive of Moodle Plugins. Search, then test one in this demo site.": "CAMP — Komunitní archiv pluginů Moodle. Vyhledejte a otestujte plugin na těchto demo stránkách.",
		"Search plugins…":   "Hledat pluginy…",
		"All types":         "Všechny typy",
		"Any status":        "Libovolný stav",
		"active":            "aktivní",
		"abandoned":         "opuštěný",
		"archived":          "archivovaný",
		"Tier":              "Úroveň",
		"All tiers":         "Všechny úrovně",
		"Claimed":           "Převzato",
		"Verified":          "Ověřeno",
		"plugins":           "pluginů",
		"installed":         "nainstalováno",
		"stars":             "hvězdy",
		"No plugins match.": "Žádné pluginy neodpovídají.",
		"Previous":          "Předchozí",
		"Next":              "Další",
		"License":           "Licence",
		"Latest on CAMP":    "Aktuální info na CAMP",
		"affected":          "zasaženo",
		"fixed in":          "opraveno ve verzi",
		"security advisory": "bezpečnostní upozornění",
		"Adding plugins is disabled on this image.":                          "Přidávání pluginů je v tomto obrazu zakázáno.",
		"Install a demo site first, then you can add plugins.":               "Nejprve nainstalujte demo stránky, poté můžete přidávat pluginy.",
		"Pull the latest community plugin catalogue from camp-registry.org.": "Stáhne nejnovější komunitní katalog pluginů z camp-registry.org.",
		"CAMP update":    "Aktualizovat CAMP",
		"Data from CAMP": "Data z CAMP",
	},
	"de": {
		"Moodle/MuTMS demo console": "Moodle/MuTMS-Demo-Konsole",

		"Demo site":  "Demo-Website",
		"Recipe":     "Rezept",
		"URL":        "URL",
		"Installed":  "Installiert",
		"tunnel":     "Tunnel",
		"Reset site": "Website zurücksetzen",
		"Cancel":     "Abbrechen",
		"Confirm":    "Bestätigen",
		"Wipe the demo site? The database, code tree and all data are deleted.": "Demo-Website wirklich löschen? Datenbank, Code und alle Daten werden entfernt.",
		"Back up data": "Daten sichern",
		"Backup name":  "Name der Sicherung",
		"The site is briefly unavailable while it is copied.": "Die Website ist während des Kopierens kurz nicht verfügbar.",
		"Quick Tunnel":                "Quick Tunnel",
		"No demo site installed yet.": "Noch keine Demo-Website installiert.",

		"Accounts":   "Konten",
		"User":       "Benutzer",
		"Password":   "Passwort",
		"Site admin": "Website-Administrator",
		"Manager":    "Manager",

		"Service": "Dienst",
		"Status":  "Status",

		"Site log":   "Website-Protokoll",
		"installing": "wird installiert",
		"resetting":  "wird zurückgesetzt",
		"backing up": "wird gesichert",
		"restoring":  "wird wiederhergestellt",
		"failed":     "fehlgeschlagen",

		"Diagnostics": "Diagnose",
		"← back":      "← zurück",

		"Site recipe":           "Website-Rezept",
		"Keep current codebase": "Aktuellen Code behalten",
		"The backup's data is loaded and upgraded to match the code it runs on.": "Die Daten der Sicherung werden geladen und an den laufenden Code angepasst.",
		"Install": "Installieren",

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

		"Settings":          "Einstellungen",
		"Catalogues":        "Kataloge",
		"Update catalogues": "Kataloge aktualisieren",
		"Pull the latest plugins and site recipes from git — new versions appear in the installer.": "Holt die neuesten Plugins und Rezepte aus Git – neue Versionen erscheinen im Installer.",

		"When you report a problem, copy the whole block below into your issue at":                                     "Wenn Sie ein Problem melden, kopieren Sie den ganzen Block unten in Ihre Meldung auf",
		"It contains service states and recent log lines, no passwords.":                                               "Er enthält Dienststatus und aktuelle Logzeilen, keine Passwörter.",
		"The exact code tree — plugins and their git sources. Share it only if your problem is about the code itself.": "Der genaue Code-Baum – Plugins und ihre Git-Quellen. Teilen Sie ihn nur, wenn Ihr Problem den Code selbst betrifft.",
		"Service problems":                 "Dienstprobleme",
		"These services are not running:":  "Diese Dienste laufen nicht:",
		"lowercase letters, digits, . _ -": "Kleinbuchstaben, Ziffern, . _ -",

		"Tools":             "Werkzeuge",
		"Redirected emails": "Umgeleitete E-Mails",
		"Installed plugins": "Installierte Plugins",
		"Recommendations":   "Empfehlungen",
		"Projects and companies the mdl-demo maintainer recommends.": "Projekte und Firmen, die der mdl-demo-Betreuer empfiehlt.",
		"Refresh the code catalogues, or grab a diagnostics report.": "Code-Kataloge aktualisieren oder einen Diagnosebericht abrufen.",
		"Public":   "Öffentlich",
		"starting": "wird gestartet",
		"Off":      "Aus",
		"Share the demo site on a public trycloudflare.com URL, so the audience can open it on their own devices.": "Teilen Sie die Demo-Website unter einer öffentlichen trycloudflare.com-Adresse, damit das Publikum sie auf eigenen Geräten öffnen kann.",
		"The extra plugins this site carries on top of core Moodle.":                                               "Die zusätzlichen Plugins dieser Website über das Moodle-Kernsystem hinaus.",
		"Every mail the site sends is caught here — nothing ever leaves the container.":                            "Jede E-Mail, die die Website sendet, wird hier abgefangen – nichts verlässt jemals den Container.",
		"Save the site to a portable file, or restore one — even into a newer Moodle.":                             "Sichern Sie die Website in eine portable Datei oder stellen Sie eine wieder her – auch in einer neueren Moodle-Version.",
		"The plugins this demo site carries on top of the ones Moodle ships with.":                                 "Die Plugins, die diese Demo-Website zusätzlich zu den mit Moodle gelieferten enthält.",
		"Component":                      "Komponente",
		"Type":                           "Typ",
		"Location":                       "Speicherort",
		"Source":                         "Quelle",
		"Select one for details.":        "Wählen Sie eines für Details.",
		"Add a plugin":                   "Plugin hinzufügen",
		"This site runs Moodle":          "Diese Website läuft mit Moodle",
		"pick a matching version below.": "wählen Sie unten eine passende Version.",
		"Show versions":                  "Versionen anzeigen",
		"Install plugin":                 "Plugin installieren",
		"Back up first — a restore point in case of breakage": "Zuerst sichern — ein Wiederherstellungspunkt bei Problemen",
		"Installing from": "Installation von",
		"Installing":      "Installiert wird",
		"Branches":        "Branches",
		"Tags":            "Tags",
		"adding plugin":   "Plugin wird hinzugefügt",
		"Adding a plugin runs its code on this throwaway demo. If things break, reset the site; if it gets compromised, delete the container entirely.": "Ein Plugin hinzuzufügen führt seinen Code auf dieser Wegwerf-Demo aus. Geht etwas kaputt, setzen Sie die Website zurück; bei einer Kompromittierung löschen Sie den Container ganz.",
		"Version":                              "Version",
		"No additional plugins installed yet.": "Noch keine zusätzlichen Plugins installiert.",

		"Pick a version to install a demo": "Wählen Sie eine Demo-Version zur Installation",
		"older versions":                   "ältere Versionen",
		"Release":                          "Release",
		"Development":                      "Entwicklung",
		"Full suite":                       "Komplettpaket",
		"On plain Moodle":                  "Reines Moodle mit Plugins",
		"Plain Moodle — core only, no plugins.":                                    "Reines Moodle – nur der Kern, keine Plugins.",
		"Plain Moodle, still in development — a look at what's coming.":            "Reines Moodle in Entwicklung – ein Blick auf das Kommende.",
		"Patched Moodle core with multi-tenancy and every MuTMS plugin.":           "Gepatchter Moodle-Kern mit Mandantenfähigkeit und allen MuTMS-Plugins.",
		"All MuTMS plugins on plain Moodle core — no multi-tenancy.":               "Alle MuTMS-Plugins auf reinem Moodle-Kern – ohne Mandantenfähigkeit.",
		"The full MuTMS suite in active development, on the latest stable Moodle.": "Das komplette MuTMS-Paket in aktiver Entwicklung, auf dem neuesten stabilen Moodle.",
		"IOMAD, a multi-tenant Moodle distribution — its current stable branches.": "IOMAD, eine mandantenfähige Moodle-Distribution – ihre aktuellen stabilen Branches.",
		"Site name": "Name der Website",

		"A backup packs the database, the uploaded files and the site's exact code recipe into one portable .mdb file.": "Eine Sicherung packt die Datenbank, die hochgeladenen Dateien und das genaue Code-Rezept der Website in eine portable .mdb-Datei.",
		"Upload": "Hochladen",
		"Upload a .mdb file created here or by another demo container.": "Laden Sie eine .mdb-Datei hoch, die hier oder von einem anderen Demo-Container erstellt wurde.",
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

		"CAMP": "CAMP",
		"Browse the community catalogue and test plugins in this demo site.":                       "Durchsuchen Sie den Community-Katalog und testen Sie Plugins auf dieser Demo-Website.",
		"CAMP — the Community Archive of Moodle Plugins. Search, then test one in this demo site.": "CAMP — Das Community-Archiv für Moodle-Plugins. Suchen und ein Plugin auf dieser Demo-Website testen.",
		"Search plugins…":   "Plugins suchen…",
		"All types":         "Alle Typen",
		"Any status":        "Beliebiger Status",
		"active":            "aktiv",
		"abandoned":         "aufgegeben",
		"archived":          "archiviert",
		"Tier":              "Stufe",
		"All tiers":         "Alle Stufen",
		"Claimed":           "Beansprucht",
		"Verified":          "Verifiziert",
		"plugins":           "Plugins",
		"installed":         "installiert",
		"stars":             "Sterne",
		"No plugins match.": "Keine passenden Plugins.",
		"Previous":          "Zurück",
		"Next":              "Weiter",
		"License":           "Lizenz",
		"Latest on CAMP":    "Aktuelles auf CAMP",
		"affected":          "betroffen",
		"fixed in":          "behoben in",
		"security advisory": "Sicherheitshinweis",
		"Adding plugins is disabled on this image.":                          "Das Hinzufügen von Plugins ist in diesem Image deaktiviert.",
		"Install a demo site first, then you can add plugins.":               "Installieren Sie zuerst eine Demo-Website, dann können Sie Plugins hinzufügen.",
		"Pull the latest community plugin catalogue from camp-registry.org.": "Ruft den neuesten Community-Plugin-Katalog von camp-registry.org ab.",
		"CAMP update":    "CAMP aktualisieren",
		"Data from CAMP": "Daten aus CAMP",
	},
}
