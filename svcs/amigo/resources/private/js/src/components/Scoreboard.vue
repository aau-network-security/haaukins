<template>
<table class="table centerbox is-striped">
  <thead>
    <tr>
      <th></th>
      <th>Team</th>
      <th v-for="chal in challenges" v-bind:key="chal.tag"><abbr :title="chal.name">{{ chal.tag }}</abbr></th>
    </tr>
  </thead>
  <tbody>
    <team-row v-for="(team, index) in teams" v-bind:key="team.id" :name="team.name" :completions="team.completions" :pos="index + 1"></team-row>
  </tbody>
</table>
</template>

<script>
import TeamRow from './TeamRow.vue'
/* eslint-disable */

export default {
  name: 'scoreboard',
  data: () => {
    return {
      teams: [],
      challenges: [],
    }
  },
  created: function() {
    var url = new URL('/scores', window.location.href);
    url.protocol = url.protocol.replace('http', 'ws');
    this.connectToWS(url.href);
  },
  methods: {
    connectToWS: function(url) {
      var self = this;
      var ws = new WebSocket(url);
      ws.onmessage = self.receiveMsg
      ws.onclose = function(){
	ws = null;
        setTimeout(function(){self.connectToWS(url)}, 3000);
      };
    },
    receiveMsg: function(evt) {
	var messages = evt.data.split('\n');
	for (var i = 0; i < messages.length; i++) {
	  const msg = messages[i];
	  const json = JSON.parse(msg);
	  switch (json.msg) {
	  case "challenges":
	    this.challenges = json.values;
	    break;
	  case "teams":
	    this.teams = json.values;
	    break;
	  }
	}
    },
  },
  components: {
    TeamRow,
  }
}
</script>
