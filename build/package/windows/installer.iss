; FoCST Inno Setup installer
#define AppName "FoCST"
#define AppPublisher "oukeidos"
#define AppId "370f6286-b2b0-4ebe-8e17-22ee03592206"

#ifndef AppVersion
  #define AppVersion "0.0.0"
#endif

#ifndef StagingDir
  #define StagingDir "dist\\stage\\windows"
#endif

[Setup]
AppId={#AppId}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
DefaultDirName={localappdata}\Programs\{#AppPublisher}\{#AppName}
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
ChangesEnvironment=yes
Compression=lzma
SolidCompression=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
SetupIconFile={#SourcePath}\..\..\assets\icon.ico
UninstallDisplayIcon={app}\focst.ico
OutputBaseFilename=FoCST_{#AppVersion}_windows_amd64
OutputDir=.

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Desktop Shortcut"; Flags: checkedonce
Name: "addpath"; Description: "Add to PATH"; Flags: checkedonce

[Files]
Source: "{#StagingDir}\focst-gui.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#StagingDir}\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#StagingDir}\*.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#StagingDir}\focst.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#StagingDir}\third_party_licenses\*"; DestDir: "{app}\third_party_licenses"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "{#StagingDir}\bin\focst.exe"; DestDir: "{app}\bin"; Flags: ignoreversion

[Icons]
Name: "{group}\FoCST"; Filename: "{app}\focst-gui.exe"; WorkingDir: "{app}"; IconFilename: "{app}\focst.ico"
Name: "{group}\Uninstall FoCST"; Filename: "{uninstallexe}"
Name: "{userdesktop}\FoCST"; Filename: "{app}\focst-gui.exe"; Tasks: desktopicon; WorkingDir: "{app}"; IconFilename: "{app}\focst.ico"

[Registry]
Root: HKCU; Subkey: "Environment"; ValueType: expandsz; ValueName: "Path"; ValueData: "{code:GetPathValue}"; Flags: preservestringtype; Tasks: addpath

[Code]
function SendMessageTimeout(hWnd: Integer; Msg: Integer; wParam: Integer; lParam: string; fuFlags: Integer; uTimeout: Integer; var lpdwResult: Integer): Integer;
  external 'SendMessageTimeoutW@user32.dll stdcall';
function PathContains(const PathValue, Entry: string): Boolean;
var
  UpperPath: string;
  UpperEntry: string;
begin
  UpperPath := Uppercase(PathValue);
  UpperEntry := Uppercase(Entry);
  Result := Pos(';' + UpperEntry + ';', ';' + UpperPath + ';') > 0;
end;

function AddPathEntry(const PathValue, Entry: string): string;
begin
  if PathValue = '' then
    Result := Entry
  else if PathContains(PathValue, Entry) then
    Result := PathValue
  else
    Result := PathValue + ';' + Entry;
end;

function RemovePathEntry(const PathValue, Entry: string): string;
var
  I: Integer;
  S: string;
  Part: string;
  NewValue: string;
  UpperEntry: string;
begin
  NewValue := '';
  UpperEntry := Uppercase(Entry);
  S := PathValue + ';';
  Part := '';
  for I := 1 to Length(S) do
  begin
    if S[I] = ';' then
    begin
      if (Part <> '') and (Uppercase(Part) <> UpperEntry) then
      begin
        if NewValue = '' then
          NewValue := Part
        else
          NewValue := NewValue + ';' + Part;
      end;
      Part := '';
    end
    else
      Part := Part + S[I];
  end;
  Result := NewValue;
end;

function GetPathValue(Param: string): string;
var
  Existing: string;
  Entry: string;
begin
  Entry := ExpandConstant('{app}\bin');
  if RegQueryStringValue(HKCU, 'Environment', 'Path', Existing) then
    Result := AddPathEntry(Existing, Entry)
  else
    Result := Entry;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  Existing: string;
  Entry: string;
begin
  if CurUninstallStep = usUninstall then
  begin
    Entry := ExpandConstant('{app}\bin');
    if RegQueryStringValue(HKCU, 'Environment', 'Path', Existing) then
    begin
      Existing := RemovePathEntry(Existing, Entry);
      RegWriteStringValue(HKCU, 'Environment', 'Path', Existing);
    end;
  end;
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  MsgResult: Integer;
  HwndBroadcast: Integer;
  WmSettingChange: Integer;
  SmtoAbortIfHung: Integer;
begin
  if CurStep = ssPostInstall then
  begin
    if WizardIsTaskSelected('addpath') then
    begin
      HwndBroadcast := $FFFF;
      WmSettingChange := $001A;
      SmtoAbortIfHung := $0002;
      SendMessageTimeout(HwndBroadcast, WmSettingChange, 0, 'Environment', SmtoAbortIfHung, 5000, MsgResult);
    end;
  end;
end;
