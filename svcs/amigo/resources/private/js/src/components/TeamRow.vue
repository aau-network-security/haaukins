<template>
<tr>
  <td>
    <span class="icon" v-bind:class="{ 'has-text-warning': pos === 1, 'is-silver': pos === 2, 'is-bronze': pos === 3 }">
      <i class="fas fa-trophy"></i>
    </span>
  </td>
  <td><strong>{{ name }}</strong></td>
  <td>
    <challenge-cell v-for="comp in completions" :completed="comp !== null"></challenge-cell>
  </td>
</tr>
</template>

<script>
import ChallengeCell from './ChallengeCell.vue'

export default {
  name: 'team-row',
  props: {
    name: String,
    pos: Number,
    completions: Array,
  },
  components: {
    ChallengeCell,
  }
}
</script>
