# bateria-hook.dll — čas priamo v hodinách panela

Táto knižnica vloží zostávajúci čas batérie **priamo do hodín** v paneli úloh
(vpravo dole pri dátume) — nie ako samostatná ikona a nie ako okno prilepené
navrch. Rovnaký princíp používa známy nástroj Windhawk.

Funguje to tak, že sa knižnica zavedie do Prieskumníka (`explorer.exe`) a
presmeruje verejné funkcie `GetLocalTime` a `GetTimeFormatEx`, ktoré panel volá
pri skladaní textu hodín. Nič sa neprepisuje v kóde Windowsu a všetko je vratné.

## Preložiť (raz)

Knižnicu treba raz preložiť. Nie je podpísaná, preto ju hlavná aplikácia
nerozdáva hotovú — musíš ju vytvoriť na svojom počítači. Je to jeden krok.

### Najjednoduchšie: MSYS2 / MinGW (zadarmo)

1. Nainštaluj **MSYS2** z https://www.msys2.org/ (klasický inštalátor, ďalej,
   ďalej).
2. Otvor „MSYS2 MINGW64" (zelená ikonka v ponuke Štart) a spusti:

   ```
   pacman -S mingw-w64-x86_64-gcc
   ```

3. Pridaj `C:\msys64\mingw64\bin` do systémovej premennej `PATH`
   (Štart → „Upraviť premenné prostredia systému" → Premenné prostredia →
   `Path` → Upraviť → Nový).
4. Dvojklikni na **`build.bat`** v tomto priečinku. Vytvorí sa `bateria-hook.dll`.

### Alternatíva: Visual Studio Build Tools (zadarmo)

1. Stiahni „Build Tools for Visual Studio" z
   https://visualstudio.microsoft.com/downloads/ (časť *Tools for Visual
   Studio* → *Build Tools*). Pri inštalácii zaškrtni **Desktop development
   with C++**.
2. Otvor **„x64 Native Tools Command Prompt for VS"** z ponuky Štart.
3. Prejdi do tohto priečinka (`cd`) a spusti `build.bat`.

## Zapnúť

1. Skopíruj hotovú **`bateria-hook.dll`** do rovnakého priečinka, kde máš
   `bateria.exe`.
2. Spusti `bateria.exe`. Čas sa objaví v hodinách vedľa dátumu.
3. Overenie: `bateria.exe -diag` napíše, či je DLL nájdená a hák zavedený.
   V ponuke pravého tlačidla je prepínač *Čas v hodinách panela*.

Preklad sa robí len raz. Odvtedy stačí `bateria.exe` a `bateria-hook.dll`
vedľa seba.

## Keby niečo

- **Antivírus/SmartScreen** môže nepodpísanú DLL alebo jej zavedenie označiť za
  podozrivé — je to očakávané pri tomto type nástroja; povoľ ju.
- **Hodiny sa správajú divne** → v Správcovi úloh reštartuj *Prieskumník
  Windows*. Po reštarte je systém v pôvodnom stave; zmeny žijú len v pamäti
  bežiaceho procesu, nič sa natrvalo nemení.
- **DLL sa nenačíta** → skontroluj, že `bateria.exe` aj `bateria-hook.dll` sú
  64-bitové a v tom istom priečinku, a spusti `bateria.exe -diag`.
