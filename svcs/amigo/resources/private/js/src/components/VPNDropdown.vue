<template>
  <div class="dropdown">
    <button class="btn btn-haaukins dropdown-toggle dropdown-css" type="button" id="dropdownMenuButton" data-toggle="dropdown"  v-on:click="createDropDown()" >
      VPN
    </button>
    <div class="dropdown-menu custom-css"  aria-labelledby="dropdownMenuButton">
      <a v-for="item in dropDownList" v-bind:key="item.vpnConnID" class="dropdown-item vpn-dd-line" v-on:click="downloadConf(item.vpnConnID,item.status)">
        {{item.vpnConnID}}
        <span class="float-right">{{item.status}}</span>
      </a>
  </div>
  </div>

</template>


<script>
const axios = require('axios').default;
export default {
  name: "VPNDropdown",
  data: () => {
    return {
      dropDownList: [],
    }
  },
  methods:  {
    createDropDown: async function() {
      this.dropDownList = []
      const opts = {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      };
      let res = await fetch('/vpn/status', opts).
      then(res => res.json());
      for (let i = 0; i <res.length ; i++) {
        this.dropDownList.push(res[i])
      }
    },
    downloadConf:  function (vpnConnID,status) {

      axios({
        url: '/vpn/download',
        method: 'POST',
        responseType: 'blob',
        data:{ vpnConnID: vpnConnID, status:status },
      }).then((response) => {
        var fileURL = window.URL.createObjectURL(new Blob([response.data]));
        var fileLink = document.createElement('a');

        fileLink.href = fileURL;
        fileLink.setAttribute('download', vpnConnID + '.conf');
        document.body.appendChild(fileLink);

        fileLink.click();
      });
    }
  }
}
</script>

<style scoped>
.vpn-dd-line{
  border-bottom: 1px solid #000;
  border-color: #211a52;
  border-radius: inherit;
}
.custom-css {
  width: 180px;
  cursor:pointer;
  color: #211a52;
}
</style>