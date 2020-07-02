// Copyright (c) Facebook, Inc. and its affiliates.
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

// @prettier
// @flow

import blue from '@material-ui/core/colors/blue';
import Button from '@material-ui/core/Button';
import Grid from '@material-ui/core/Grid';
import Menu from '@material-ui/core/Menu';
import MenuItem from '@material-ui/core/MenuItem';
import Modal from '@material-ui/core/Modal';
import React from 'react';
import TextField from '@material-ui/core/TextField';

import {APIUtil} from './APIUtil';
import {makeStyles} from '@material-ui/core/styles';
import {useState} from 'react';

import type {ApiUtil} from '@fbcnms/alarms/components/AlarmsApi';

const useStyles = makeStyles((theme) => ({
  modal: {
    position: 'absolute',
    width: 400,
    backgroundColor: theme.palette.background.paper,
    padding: theme.spacing(2, 4, 3),
    top: `50%`,
    left: `50%`,
    transform: `translate(-50%, -50%)`,
  },
  tenantMenu: {
    position: 'fixed',
    bottom: 5,
    left: 5,
    backgroundColor: blue[200],
  },
}));


function CreateTenantModal(props: {open: boolean, setTenantId: (string) => void, onClose: () => void}) {
  const classes = useStyles();
  const [newTenant, setNewTenant] = useState("");

  const handleSave = () => {
    if (newTenant !== "") {
      props.setTenantId(newTenant);
    }
    APIUtil(newTenant).editRouteTree({
      route: {
        receiver: `${newTenant}_tenant_base_route`,
      }
    })
    props.onClose();
  }

  const body = (
    <div className={classes.modal}>
      <h2>New Tenant</h2>
      <Grid container>
        <TextField label="Tenant ID" variant="outlined" onChange={(e) => setNewTenant(e.target.value)} />
        <Button onClick={handleSave}> Save </Button>
      </Grid>
    </div>
  );

  return (
    <Modal
      open={props.open}
      onClose={props.onClose}
    >
      {body}
    </Modal>
  );
}

export default function TenantSelector(props: {apiUtil: ApiUtil, setTenantId: (string) => void, tenantID: string}) {
  const classes = useStyles();
  const [anchorEl, setAnchorEl] = React.useState(null);
  const [open, setOpen] = useState(false);

  const {response} = props.apiUtil.useAlarmsApi(
    props.apiUtil.getTenants,
    {open},
  );
  const tenantList = response ?? [];

  const handleClose = (event) => {
    if (event.target.dataset.tenantId) {
      props.setTenantId(event.target.dataset.tenantId)
    }
    setAnchorEl(null);
  }

  const handleModalClose = () => {
    setAnchorEl(null);
    setOpen(false);
  }

  return (
    <>
      <Button className={classes.tenantMenu} aria-controls="tenantMenu" aria-haspopup="true" onClick={e => setAnchorEl(e.currentTarget)}>
        {`Tenant: ${props.tenantID}`}
      </Button>
      <Menu
        anchorEl={anchorEl}
        keepMounted
        open={Boolean(anchorEl)}
        onClose={handleClose}
        onClick={handleClose}
      >
      {tenantList.map(tenant => {
        return <MenuItem key={tenant} data-tenant-id={tenant}>{tenant}</MenuItem>;
      })}
        <MenuItem>
          <Button onClick={() => setOpen(true)}>
            Create Tenant
          </Button>
        </MenuItem>
      </Menu>
      <CreateTenantModal open={open} setTenantId={props.setTenantId} onClose={handleModalClose}/>
    </>
  )
}
