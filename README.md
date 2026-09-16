# Batéria

Ikona batérie v oznamovacej oblasti Windowsu (vpravo dole pri hodinách), ktorá
navyše ukazuje **čas do plného nabitia** a **čas do vybitia**.

Vyzerá zhruba ako vstavaná ikona Windowsu, ale po nabehnutí myšou a po
kliknutí povie to, čo Windows sám nepovie.

![Ikony vo všetkých stavoch](docs/ikony.png)

*Hore: režim „batéria“ na tmavom a svetlom paneli. Dole: režim „percentá“.*

## Čo to robí

**Bublina pri ikone** (stačí nabehnúť myšou):

```
73 % – nabíja sa
Do plného nabitia: 1 h 12 min
Nabíjanie: 24,5 W
```

Na batérii sa namiesto toho ukáže `Do vybitia: 2 h 40 min`.

**Kliknutie ľavým tlačidlom** otvorí okno s podrobnosťami: percentá, stav,
zostávajúci čas, kapacita v mWh, zdravie batérie (súčasná plná kapacita oproti
návrhovej), počet nabíjacích cyklov, napätie a zdroj odhadu. Keď je počítač
v sieti, pribudne riadok **Výdrž po odpojení** – odhad podľa toho, ako rýchlo
sa batéria vybíjala naposledy.

**Kliknutie pravým tlačidlom** otvorí ponuku: prepnutie vzhľadu ikony
(obrys batérie / percentá), spúšťanie s Windowsom a ukončenie.

Ikona sa sama prispôsobí svetlému aj tmavému panelu úloh a mierke DPI
(na obrazovkách so 150 % sa vykreslí vo vyššom rozlíšení, nie rozmazane).
Pri nabití pod 20 % zožltne, pod 10 % sčervenie.

## Odkiaľ sa berie čas do plného nabitia

Toto je jadro celej aplikácie. Windows cez `GetSystemPowerStatus` hlási len
čas do vybitia (`BatteryLifeTime`) – a čas do plného nabitia
(`BatteryFullLifeTime`) v praxi vracia „neznáme“. Preto aplikácia siaha
priamo na ovládač batérie:

1. **Ovládač batérie** (`IOCTL_BATTERY_QUERY_STATUS` cez `setupapi`) – dá
   okamžitý tok energie v mW aj kapacitu v mWh. Čas do plného nabitia je potom
   `(plná kapacita − aktuálna) / tok`, čas do vybitia `aktuálna / tok`.
   Okamžitá hodnota kolíše podľa zaťaženia počítača, preto sa vyhladzuje
   exponenciálnym priemerom s časovou konštantou 45 sekúnd.
2. **Vlastné meranie v čase** – keď ovládač tok nehlási (stáva sa to), použije
   sa smernica priamky preloženej meraniami za posledných 12 minút
   (metóda najmenších štvorcov).
3. **Odhad Windowsu** – posledná záchrana, funguje len pre čas do vybitia.

Ktorý zdroj sa práve použil, je vidieť v okne s podrobnosťami v riadku
*Zdroj odhadu*. Nezmyselné výsledky (kratšie než 30 sekúnd alebo dlhšie než
72 hodín) sa zahadzujú a pri zmene stavu (pripojenie nabíjačky) sa história
meraní vynuluje, aby sa nemiešali dva rôzne režimy.

Prvú minútu po spustení môže byť namiesto času napísané `Čas sa ešte počíta…`
– kým nie sú aspoň tri merania z minimálne 90 sekúnd.

## Zostavenie

Treba [Go 1.22+](https://go.dev/dl/). Žiadne ďalšie knižnice – projekt nemá
jedinú externú závislosť, takže `go build` funguje aj bez internetu.

Vo Windowse:

```powershell
powershell -ExecutionPolicy Bypass -File build.ps1
```

Alebo priamo:

```powershell
go build -trimpath -ldflags "-H=windowsgui -s -w" -o dist\bateria.exe .\cmd\bateria
```

`-H=windowsgui` je dôležité – bez neho by sa pri každom spustení otvorilo aj
čierne okno konzoly.

Na Linuxe alebo macOS sa dá `.exe` pripraviť tiež (`make build`), Go vie
prekladať pre iný systém.

## Spustenie

Stačí spustiť `bateria.exe`. Ikona sa objaví v paneli úloh – ak ju Windows
schová do prepadovej ponuky (šípka `^`), dá sa vytiahnuť potiahnutím myšou
alebo cez *Nastavenia → Prispôsobenie → Panel úloh → Ikony v rohu panela úloh*.

Aby sa aplikácia spúšťala po prihlásení, stačí v ponuke pravého tlačidla
zapnúť **Spúšťať s Windowsom** (zapíše sa hodnota do
`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`).

Naraz beží len jedna inštancia; druhé spustenie sa ticho ukončí.

### Vstavaná ikona Windowsu

Systémovú ikonu batérie nahradiť ani upraviť nejde – je súčasťou Prieskumníka.
Táto aplikácia je samostatná ikona vedľa nej. Vo Windowse 10 sa tá pôvodná dá
vypnúť v *Nastavenia → Prispôsobenie → Panel úloh → Zapnutie alebo vypnutie
systémových ikon → Napájanie*. Windows 11 túto možnosť v nastaveniach nemá.

## Nastavenia

Uložené sú v `%APPDATA%\Bateria\config.json`:

```json
{
  "icon_mode": "battery",
  "refresh_seconds": 2
}
```

* `icon_mode` – `battery` (obrys batérie) alebo `percent` (číslo v paneli).
  Prepína sa aj v ponuke pravého tlačidla.
* `refresh_seconds` – ako často sa meria (1 až 60 sekúnd).

Pri poškodenom súbore sa použijú predvolené hodnoty; aplikácia sa kvôli
nastaveniam nikdy nezastaví.

## Ako je to poskladané

```
cmd/bateria      spustiteľný program
cmd/icongen      vygeneruje assets/app.ico a docs/ikony.png zo zdrojáku ikony
internal/battery stav batérie + odhady časov (jadro, plne otestované)
internal/icon    kreslenie ikony (SDF, vyhladené hrany, ostré číslice)
internal/config  nastavenia
internal/win     tenká vrstva nad Win32 API (bez externých závislostí)
internal/winui   ikona v paneli, ponuka, okno s podrobnosťami
```

Logika odhadov, kreslenie ikony aj texty sú oddelené od Win32 kódu, takže sa
dajú testovať kdekoľvek:

```
go test ./...
```

## Známe obmedzenia

* Niektoré ovládače hlásia kapacity v bezrozmerných jednotkách namiesto mWh.
  Časy sú aj vtedy správne (jednotky sa vykrátia), ale watty a kapacita sa
  neukážu.
* Odhad **Výdrž po odpojení** sa objaví, až keď aplikácia aspoň chvíľu videla
  beh na batérii – dovtedy nemá z čoho počítať.
* Pri dvoch a viac batériách sa kapacity aj tok spočítajú dohromady; systém
  ich aj tak používa ako jeden zdroj.
