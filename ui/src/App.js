// @prettier
// @flow

import React from 'react';
import amber from '@material-ui/core/colors/amber';
import green from '@material-ui/core/colors/green';
import MuiStylesThemeProvider from '@material-ui/styles/ThemeProvider';
import {BrowserRouter, Redirect, Route} from 'react-router-dom';
import {APIUtil} from './APIUtil';
import {Alarms} from '@fbcnms/alarms';
import {createMuiTheme} from '@material-ui/core/styles';
import {MuiThemeProvider} from '@material-ui/core/styles';
import {SnackbarProvider} from 'notistack';


// default theme
const theme = createMuiTheme({
  palette: {
    success: {
      light: green[100],
      main: green[500],
      dark: green[800],
    },
    warning: {
      light: amber[100],
      main: amber[500],
      dark: amber[800],
    },
    // symphony theming
    secondary: {
      main: '#303846',
    },
    grey: {
      '50': '#e4f0f6',
    },
  },
});

function AlarmsMain() {
  return(
    <>
      <MuiThemeProvider theme={theme}>
        <MuiStylesThemeProvider theme={theme}>
          <SnackbarProvider>
          <Alarms
            apiUtil={APIUtil}
            makeTabLink={({match, keyName}) => {
                return `${keyName}`
              }
            }
            alertManagerGlobalConfigEnabled={true}
            disabledTabs={['suppressions', 'routes']}
          />
          </SnackbarProvider>
        </MuiStylesThemeProvider>
      </MuiThemeProvider>
    </>
  )
}

function App() {
  return (
    <div>
      <BrowserRouter >
        <Route path={"/alarms"} exact={false} render={() => <AlarmsMain />}>
        </Route>
        <Redirect to={"/alarms"}></Redirect>
      </BrowserRouter>
    </div>
  );
}

export default App;
