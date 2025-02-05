# What's next?

Run the following commands to get started:

    *`cd ./grafana-openweather-datasource`
    * `npm install` to install frontend dependencies.
    * `npm exec playwright install chromium` to install e2e test dependencies.
    * `npm run dev` to build (and watch) the plugin frontend code.
    * `mage -v build:linux` to build the plugin backend code. Rerun this command every time you edit your backend files.
    * `docker compose up` to start a grafana development server.
    * Open http://localhost:3000 in your browser to create a dashboard to begin developing your plugin.

Note: We strongly recommend creating a new Git repository by running git init in ./grafana-openweather-datasource before continuing.

    * Learn more about Grafana Plugin Development at https://grafana.com/developers/plugin-tools


### **1. Überprüfe, ob `mage` installiert ist**

Führe diesen Befehl aus:

`which mage
`


### **2. `mage` installieren**

Falls `mage` fehlt, installiere es mit:

`go install github.com/magefile/mage@latest`



Nach der Installation sollte sich `mage` unter `$(go env GOPATH)/bin` befinden.

Überprüfe, ob `mage` jetzt gefunden wird:

`which mage `

Falls `mage` immer noch nicht gefunden wird, füge den Go-Binärordner zu deinem `PATH` hinzu:


`export PATH=$PATH:$(go env GOPATH)/bin `


Falls du das dauerhaft machen möchtest, speichere es in deiner `.bashrc` oder `.zshrc` Datei:

`echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
source ~/.bashrc
`


### **3. `mage`-Build erneut ausführen**

Nachdem `mage` erfolgreich installiert wurde, teste den Build erneut:

`mage -v build:linux`
