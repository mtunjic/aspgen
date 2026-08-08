
@REM aspgen.exe.exe new WpfUiDemo --app fullstack --backend ddd --theme wpfui --theme-mode light --output ./WpfUiDemo


@REM aspgen.exe.exe add entity Product name:string price:decimal active:bool category:string --project ./WpfUiDemo


@REM aspgen.exe.exe add entity-field Product notes:string --project ./WpfUiDemo


@REM aspgen.exe.exe add module Reporting --project ./WpfUiDemo


@REM dotnet restore ./WpfUiDemo/WpfUiDemo.sln
@REM dotnet build ./WpfUiDemo/WpfUiDemo.sln
@REM dotnet run --project ./WpfUiDemo/src/WebApi
@REM dotnet run --project ./WpfUiDemo/src/Desktop


@REM dotnet restore ./WpfUiDemo/WpfUiDemo.sln -s https://api.nuget.org/v3/index.json


aspgen.exe new WpfLocalDemo --app wpf --backend ddd --theme wpfui --theme-mode light --output ./WpfLocalDemo
aspgen.exe add entity Product name:string price:decimal active:bool category:string --project ./WpfLocalDemo
dotnet restore ./WpfLocalDemo/WpfLocalDemo.sln
dotnet build ./WpfLocalDemo/WpfLocalDemo.sln
dotnet run --project ./WpfLocalDemo/src/Desktop