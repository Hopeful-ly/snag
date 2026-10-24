; Per-user installer: no administrator needed.
;
;   makensis -DVERSION=1.2.0 -DSOURCE=../../dist/windows/Snag -DOUTFILE=Snag-1.2.0-setup.exe snag.nsi
;
; Paths are relative to this file. SOURCE holds snag.exe, snag-host.exe and tools\.

Unicode true
SetCompressor /SOLID lzma
RequestExecutionLevel user

!include "MUI2.nsh"
!include "LogicLib.nsh"

!ifndef VERSION
  !define VERSION "0.0.0"
!endif
!define UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\Snag"

Name "Snag"
OutFile "${OUTFILE}"
InstallDir "$LOCALAPPDATA\Programs\Snag"
InstallDirRegKey HKCU "${UNINSTALL_KEY}" "InstallLocation"
BrandingText "Snag ${VERSION}"

VIProductVersion "${VERSION}.0"
VIAddVersionKey "ProductName" "Snag"
VIAddVersionKey "ProductVersion" "${VERSION}"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "FileDescription" "Snag installer"
VIAddVersionKey "LegalCopyright" "Snag"

!define MUI_ICON "../icons/snag.ico"
!define MUI_UNICON "../icons/snag.ico"
!define MUI_ABORTWARNING
!define MUI_FINISHPAGE_RUN "$INSTDIR\snag.exe"
!define MUI_FINISHPAGE_RUN_TEXT "Start Snag"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

; A running Snag holds its files: it is stopped first (downloads pick up where they were next time).
!macro StopSnag
  nsExec::Exec 'taskkill /F /IM snag.exe'
  nsExec::Exec 'taskkill /F /IM snag-host.exe'
  Sleep 500
!macroend

Section "Snag" Main
  SectionIn RO
  !insertmacro StopSnag

  SetOutPath "$INSTDIR"
  File "${SOURCE}/snag.exe"
  File "${SOURCE}/snag-host.exe"
  File "../icons/snag.ico"
  SetOutPath "$INSTDIR\tools"
  File /r "${SOURCE}/tools/*.*"
  SetOutPath "$INSTDIR"

  ; Snag's window is a web view: Windows 10 and 11 have its runtime, older setups may not.
  Call WebView2

  ; The browser extension finds Snag through this registration.
  nsExec::Exec '"$INSTDIR\snag.exe" --register-browsers'

  CreateShortCut "$SMPROGRAMS\Snag.lnk" "$INSTDIR\snag.exe" "" "$INSTDIR\snag.ico"

  WriteUninstaller "$INSTDIR\uninstall.exe"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayName" "Snag"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayIcon" "$INSTDIR\snag.ico"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegStr HKCU "${UNINSTALL_KEY}" "QuietUninstallString" '"$INSTDIR\uninstall.exe" /S'
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoRepair" 1
SectionEnd

Function WebView2
  ReadRegStr $0 HKLM "SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
  ${If} $0 == ""
    ReadRegStr $0 HKCU "Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
  ${EndIf}
  ${If} $0 == ""
  ${OrIf} $0 == "0.0.0.0"
    DetailPrint "Installing the WebView2 runtime"
    ; NSIS's own downloader can't do HTTPS; PowerShell, part of every Windows Snag runs on, can.
    nsExec::Exec `powershell -NoProfile -ExecutionPolicy Bypass -Command "Invoke-WebRequest -UseBasicParsing -Uri 'https://go.microsoft.com/fwlink/p/?LinkId=2124703' -OutFile '$TEMP\MicrosoftEdgeWebview2Setup.exe'"`
    ExecWait '"$TEMP\MicrosoftEdgeWebview2Setup.exe" /silent /install'
    Delete "$TEMP\MicrosoftEdgeWebview2Setup.exe"
  ${EndIf}
FunctionEnd

Section "Uninstall"
  !insertmacro StopSnag
  nsExec::Exec '"$INSTDIR\snag.exe" --unregister-browsers'
  Delete "$SMPROGRAMS\Snag.lnk"
  RMDir /r "$INSTDIR\tools"
  Delete "$INSTDIR\snag.exe"
  Delete "$INSTDIR\snag-host.exe"
  Delete "$INSTDIR\snag.ico"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  DeleteRegKey HKCU "${UNINSTALL_KEY}"
  ; Settings, the download list and downloaded files stay: they are the user's.
SectionEnd
