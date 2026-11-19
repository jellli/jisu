; Hooks of the Windows installer (mygo.json windows.installerInclude).
;
; jisu shows its interface in WebView2. Windows 11 ships the WebView2
; Runtime; Windows 10 may not have it. customInstall installs it, per user
; when the installer is not elevated, with Microsoft's Evergreen
; Bootstrapper: the copy CI embeds next to this file, or else one it
; downloads.

!include LogicLib.nsh

!define JISU_WEBVIEW2_GUID "{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"
!define JISU_WEBVIEW2_SETUP "${__FILEDIR__}\MicrosoftEdgeWebview2Setup.exe"
!define JISU_WEBVIEW2_URL "https://go.microsoft.com/fwlink/p/?LinkId=2124703"

!macro jisuWebView2Version hive key
  ${If} $0 == ""
  ${OrIf} $0 == "0.0.0.0"
    ReadRegStr $0 ${hive} "${key}\${JISU_WEBVIEW2_GUID}" "pv"
  ${EndIf}
!macroend

!macro customInstall
  Push $0
  Push $1
  StrCpy $0 ""
  !insertmacro jisuWebView2Version HKLM "SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients"
  !insertmacro jisuWebView2Version HKLM "SOFTWARE\Microsoft\EdgeUpdate\Clients"
  !insertmacro jisuWebView2Version HKCU "Software\Microsoft\EdgeUpdate\Clients"
  ${If} $0 == ""
  ${OrIf} $0 == "0.0.0.0"
    DetailPrint "Installing the Microsoft Edge WebView2 Runtime"
    InitPluginsDir
    !if /FileExists "${JISU_WEBVIEW2_SETUP}"
      File "/oname=$PLUGINSDIR\MicrosoftEdgeWebview2Setup.exe" "${JISU_WEBVIEW2_SETUP}"
    !else
      System::Call 'Kernel32::SetEnvironmentVariableW(w "JISU_WEBVIEW2_BOOTSTRAPPER", w "$PLUGINSDIR\MicrosoftEdgeWebview2Setup.exe") i .r1'
      ${If} $1 != 0
        nsExec::ExecToLog `"$SYSDIR\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -NonInteractive -Command "$$ErrorActionPreference = 'Stop'; [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; $$out = $$env:JISU_WEBVIEW2_BOOTSTRAPPER; Invoke-WebRequest -UseBasicParsing -Uri '${JISU_WEBVIEW2_URL}' -OutFile $$out; $$sig = Get-AuthenticodeSignature -LiteralPath $$out; if ($$sig.Status -ne 'Valid' -or $$sig.SignerCertificate.Subject -notmatch 'O=Microsoft Corporation') { exit 1 }"`
        Pop $1
        System::Call 'Kernel32::SetEnvironmentVariableW(w "JISU_WEBVIEW2_BOOTSTRAPPER", p 0)'
      ${Else}
        StrCpy $1 "error"
      ${EndIf}
      ${If} $1 != "0"
        Delete "$PLUGINSDIR\MicrosoftEdgeWebview2Setup.exe"
      ${EndIf}
    !endif
    StrCpy $1 "missing"
    ${If} ${FileExists} "$PLUGINSDIR\MicrosoftEdgeWebview2Setup.exe"
      ExecWait '"$PLUGINSDIR\MicrosoftEdgeWebview2Setup.exe" /silent /install' $1
    ${EndIf}
    ${If} $1 != 0
      DetailPrint "WebView2 Runtime setup failed: $1"
      MessageBox MB_ICONEXCLAMATION|MB_OK "jisu needs the Microsoft Edge WebView2 Runtime, which could not be installed. Install it from https://developer.microsoft.com/microsoft-edge/webview2/ and start jisu again." /SD IDOK
    ${EndIf}
  ${EndIf}
  Pop $1
  Pop $0
!macroend
